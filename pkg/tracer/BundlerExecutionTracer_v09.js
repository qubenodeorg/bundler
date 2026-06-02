// BundlerExecutionTracer for ERC-4337 v0.9 EntryPoint.
// v0.8 + capture getCurrentUserOpHash() calls made via STATICCALL to the EntryPoint
// during execution. The EntryPoint address is injected at parse time by load.go.

var tracer = {
  reverts: [],
  validationOOG: false,
  executionOOG: false,
  executionGasLimit: 0,
  currentUserOpHashes: [],

  _depth: 0,
  _executionGasStack: [],
  _defaultGasItem: { used: 0, required: 0 },
  _marker: 0,
  _validationMarker: 1,
  _executionMarker: 3,
  _userOperationEventTopics0:
    "0x49628fd1471006c1482da88028e9ce4dbb080b815c9b0344d39e5a8e6ec1419f",
  // Selector of getCurrentUserOpHash() = keccak("getCurrentUserOpHash()")[:4]
  _getCurrentUserOpHashSelector: "0x60e6b9fb",

  _entryPointAddressHex: "__ENTRY_POINT_ADDRESS_HEX__",

  _isValidation: function () {
    return (
      this._marker >= this._validationMarker &&
      this._marker < this._executionMarker
    );
  },

  _isExecution: function () {
    return this._marker === this._executionMarker;
  },

  _isUserOperationEvent: function (log) {
    var topics0 = "0x" + log.stack.peek(2).toString(16);
    return topics0 === this._userOperationEventTopics0;
  },

  _setUserOperationEvent: function (opcode, log) {
    var count = parseInt(opcode.substring(3));
    var ofs = parseInt(log.stack.peek(0).toString());
    var len = parseInt(log.stack.peek(1).toString());
    var topics = [];
    for (var i = 0; i < count; i++) {
      topics.push("0x" + log.stack.peek(2 + i).toString(16));
    }
    var data = toHex(log.memory.slice(ofs, ofs + len));
    this.userOperationEvent = {
      topics: topics,
      data: data,
    };
  },

  // Read the return value pushed onto the EVM stack at the next RETURNDATACOPY
  // boundary for STATICCALL with selector getCurrentUserOpHash. We do a
  // best-effort capture by inspecting the memory at the moment of the call.
  _maybeCaptureCurrentUserOpHash: function (log) {
    var opcode = log.op.toString();
    if (opcode !== "STATICCALL") return;
    // STATICCALL stack: gas(0), addr(1), argsOffset(2), argsLength(3), retOffset(4), retLength(5)
    var addr = toAddress(log.stack.peek(1).toString(16));
    if (toHex(addr) !== this._entryPointAddressHex) return;
    var _ofs = parseInt(log.stack.peek(2).toString());
    var _len = parseInt(log.stack.peek(3).toString());
    if (_len < 4) return;
    var inputHex = toHex(log.memory.slice(_ofs, _ofs + 4));
    if (inputHex !== this._getCurrentUserOpHashSelector) return;
    // Result is written to retOffset/retLength on the call's RETURN; we record
    // the request now and the result will be picked up on RETURN.
    this._pendingGetCurrentUserOpHash = {
      retOffset: parseInt(log.stack.peek(4).toString()),
      retLength: parseInt(log.stack.peek(5).toString()),
    };
  },

  _consumePendingGetCurrentUserOpHash: function (log) {
    if (!this._pendingGetCurrentUserOpHash) return;
    var pending = this._pendingGetCurrentUserOpHash;
    if (pending.retLength <= 0) {
      this._pendingGetCurrentUserOpHash = null;
      return;
    }
    var mem = toHex(
      log.memory.slice(pending.retOffset, pending.retOffset + pending.retLength)
    );
    this.currentUserOpHashes.push(mem);
    this._pendingGetCurrentUserOpHash = null;
  },

  fault: function fault(log, db) {},
  result: function result(ctx, db) {
    var out = {
      reverts: this.reverts,
      validationOOG: this.validationOOG,
      executionOOG: this.executionOOG,
      executionGasLimit: this.executionGasLimit,
      userOperationEvent: this.userOperationEvent,
      currentUserOpHashes: this.currentUserOpHashes,
      output: toHex(ctx.output),
      error: ctx.error,
    };
    if (this.currentUserOpHashes.length === 1) {
      out.currentUserOpHash = this.currentUserOpHashes[0];
    }
    return out;
  },

  enter: function enter(frame) {
    if (this._isExecution()) {
      var next = this._depth + 1;
      if (this._executionGasStack[next] === undefined)
        this._executionGasStack[next] = Object.assign({}, this._defaultGasItem);
    }
  },
  exit: function exit(frame) {
    if (this._isExecution()) {
      if (frame.getError() !== undefined) {
        this.reverts.push(toHex(frame.getOutput()));
        this._consumePendingGetCurrentUserOpHash({ memory: { slice: function () { return ""; } } });
      }

      if (this._depth >= 2) {
        var nested = Object.assign(
          {},
          this._executionGasStack[this._depth + 1] || this._defaultGasItem
        );

        this._executionGasStack[this._depth + 1] = Object.assign(
          {},
          this._defaultGasItem
        );

        var used = frame.getGasUsed();
        this._executionGasStack[this._depth].used += used;

        this._executionGasStack[this._depth].required +=
          used - nested.used + Math.ceil((nested.required * 64) / 63);

        this.executionGasLimit = this._executionGasStack[this._depth].required;
      }
    }
  },

  step: function step(log, db) {
    var opcode = log.op.toString();
    this._depth = log.getDepth();
    if (this._depth === 1 && opcode === "NUMBER") this._marker++;

    if (
      this._depth <= 2 &&
      opcode.startsWith("LOG") &&
      this._isUserOperationEvent(log)
    )
      this._setUserOperationEvent(opcode, log);

    if (log.getGas() < log.getCost() && this._isValidation())
      this.validationOOG = true;

    if (log.getGas() < log.getCost() && this._isExecution())
      this.executionOOG = true;

    if (this._isExecution()) {
      if (opcode === "STATICCALL") {
        this._maybeCaptureCurrentUserOpHash(log);
      } else if (opcode === "RETURN" && this._pendingGetCurrentUserOpHash) {
        this._consumePendingGetCurrentUserOpHash(log);
      }
    }
  },
};
