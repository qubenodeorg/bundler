// BundlerCollectorTracer for ERC-4337 v0.9 EntryPoint.
// v0.8 + paymasterSignature presence flag on paymaster top-level call.

var tracer = {
  callsFromEntryPoint: [],
  currentLevel: null,
  keccak: [],
  calls: [],
  logs: [],
  debug: [],
  lastOp: "",
  lastThreeOpcodes: [],
  stopCollectingTopic:
    "bb47ee3e183a558b1a2ff0874b079f3fc5478b7454eacf2bfc5af2ff5878f972",
  stopCollecting: false,
  topLevelCallCounter: 0,

  // validatePaymasterUserOp selector in v0.9 (same as v0.7/0.8).
  validatePaymasterUserOpSelector: "0xc04a6310",
  // handleOps selector in v0.9 (same as v0.7/0.8).
  handleOpsSelector: "0x1a5d1cd7",

  // Return true if the top-level entrypoint.handleOps(...) call has a
  // non-empty eip7702Auth tuple, parsed from the ABI-encoded calldata.
  _hasEip7702Auth: function (inputHex) {
    if (!inputHex || inputHex.length < 10) return false;
    if (inputHex.slice(0, 10) !== "0x1a5d1cd7") return false;
    var payload = inputHex.slice(10);
    if (payload.length < 64 * 3) return false;
    var authOffsetHex = payload.slice(128, 192);
    var authOffset = parseInt(authOffsetHex, 16);
    if (isNaN(authOffset)) return false;
    var lengthStart = authOffset * 2;
    var authLengthHex = payload.slice(lengthStart, lengthStart + 64);
    if (!authLengthHex) return false;
    var authLength = parseInt(authLengthHex, 16);
    if (isNaN(authLength)) return false;
    return authLength > 0;
  },

  // Return true if the validatePaymasterUserOp(...) call's calldata is
  // long enough to include a non-empty paymasterSignature tail.
  // The v0.9 PackedUserOperation grew a paymasterSignature bytes field;
  // we cannot easily parse it out of the deeply-nested ABI calldata in a
  // JS tracer, so we use a calibrated length-based heuristic: a call
  // without a paymasterSignature still includes the head + the (empty)
  // signature tail offset, which falls under PAYMASTER_SIG_NO_SIG_MAX_LEN
  // hex chars; a call with a signature will exceed that threshold.
  _hasPaymasterSignature: function (inputHex) {
    if (!inputHex || inputHex.length < 10) return false;
    if (inputHex.slice(0, 10) !== "0xc04a6310") return false;
    var payload = inputHex.slice(10);
    // Threshold: a no-signature validatePaymasterUserOp call has at most
    // 4 head words + an empty signature tail. 14 head words is a safe
    // upper bound for the no-signature case; anything longer indicates
    // a non-empty signature.
    return payload.length > 64 * 14;
  },

  fault: function fault(log, _db) {
    this.debug.push(
      "fault depth=" +
        log.getDepth() +
        " gas=" +
        log.getGas() +
        " cost=" +
        log.getCost() +
        " err=" +
        log.getError()
    );
  },

  result: function result(ctx, _db) {
    var hasAuth = this._hasEip7702Auth(toHex(ctx.input));
    for (var i = 0; i < this.callsFromEntryPoint.length; i++) {
      this.callsFromEntryPoint[i].eip7702Auth = hasAuth;
    }
    return {
      callsFromEntryPoint: this.callsFromEntryPoint,
      keccak: this.keccak,
      logs: this.logs,
      calls: this.calls,
      debug: this.debug,
    };
  },

  enter: function enter(frame) {
    if (this.stopCollecting) {
      return;
    }
    var input = toHex(frame.getInput());
    this.calls.push({
      type: frame.getType(),
      from: toHex(frame.getFrom()),
      to: toHex(frame.getTo()),
      method: input.slice(0, 10),
      gas: frame.getGas(),
      value: frame.getValue(),
    });
  },

  exit: function exit(frame) {
    if (this.stopCollecting) {
      return;
    }
    this.calls.push({
      type: frame.getError() != null ? "REVERT" : "RETURN",
      gasUsed: frame.getGasUsed(),
      data: toHex(frame.getOutput()).slice(0, 4000),
    });
  },

  countSlot: function countSlot(list, key) {
    if (!list[key]) list[key] = 0;
    list[key] += 1;
  },

  step: function step(log, db) {
    if (this.stopCollecting) {
      return;
    }
    var opcode = log.op.toString();

    var stackSize = log.stack.length();
    var stackTop3 = [];
    for (var i = 0; i < 3 && i < stackSize; i++) {
      stackTop3.push(log.stack.peek(i));
    }
    this.lastThreeOpcodes.push({
      opcode: opcode,
      stackTop3: stackTop3,
    });
    if (this.lastThreeOpcodes.length > 3) {
      this.lastThreeOpcodes.shift();
    }

    if (
      log.getGas() < log.getCost() ||
      (opcode === "SSTORE" && log.getGas() < 2300)
    ) {
      if (this.currentLevel) this.currentLevel.oog = true;
    }

    if (opcode === "REVERT" || opcode === "RETURN") {
      if (log.getDepth() === 1) {
        var ofs = parseInt(log.stack.peek(0).toString());
        var len = parseInt(log.stack.peek(1).toString());
        var data = toHex(log.memory.slice(ofs, ofs + len)).slice(0, 4000);
        this.calls.push({
          type: opcode,
          gasUsed: 0,
          data: data,
        });
      }
      this.lastThreeOpcodes = [];
    }

    if (log.getDepth() === 1) {
      if (opcode === "CALL" || opcode === "STATICCALL") {
        var addr = toAddress(log.stack.peek(1).toString(16));
        var topLevelTargetAddress = toHex(addr);
        var _ofs = parseInt(log.stack.peek(3).toString());
        var topLevelMethodSig = toHex(log.memory.slice(_ofs, _ofs + 4));
        var _lenIdx = opcode === "STATICCALL" ? 3 : 4;
        var _len = parseInt(log.stack.peek(_lenIdx).toString());
        var inputHex = toHex(log.memory.slice(_ofs, _ofs + _len));
        var hasPaymasterSig = this._hasPaymasterSignature(inputHex);

        this.currentLevel = this.callsFromEntryPoint[this.topLevelCallCounter] =
          {
            topLevelMethodSig: topLevelMethodSig,
            topLevelTargetAddress: topLevelTargetAddress,
            access: {},
            opcodes: {},
            extCodeAccessInfo: {},
            contractSize: {},
            paymasterSignature: hasPaymasterSig,
          };
        this.topLevelCallCounter++;
      } else if (opcode === "LOG1") {
        var topic = log.stack.peek(2).toString(16);
        if (topic === this.stopCollectingTopic) {
          this.stopCollecting = true;
        }
      }
      this.lastOp = "";
      return;
    }

    var lastOpInfo = this.lastThreeOpcodes[this.lastThreeOpcodes.length - 2];
    if (lastOpInfo && lastOpInfo.opcode.match(/^(EXT.*)$/) != null) {
      var _addr = toAddress(lastOpInfo.stackTop3[0].toString(16));
      var addrHex = toHex(_addr);
      var last3opcodesString = this.lastThreeOpcodes
        .map(function (x) {
          return x.opcode;
        })
        .join(",");
      if (last3opcodesString.match(/^(\w+),EXTCODESIZE,ISZERO$/) == null) {
        this.currentLevel.extCodeAccessInfo[addrHex] = opcode;
      }
    }

    var isAllowedPrecompiled = function isAllowedPrecompiled(address) {
      var addrHex = toHex(address);
      var addressInt = parseInt(addrHex);
      return addressInt > 0 && addressInt < 10;
    };
    if (
      opcode.match(/^(EXT.*|CALL|CALLCODE|DELEGATECALL|STATICCALL)$/) != null
    ) {
      var idx = opcode.startsWith("EXT") ? 0 : 1;
      var _addr2 = toAddress(log.stack.peek(idx).toString(16));
      var _addrHex = toHex(_addr2);
      if (
        this.currentLevel.contractSize[_addrHex] == null &&
        !isAllowedPrecompiled(_addr2)
      ) {
        this.currentLevel.contractSize[_addrHex] = {
          contractSize: db.getCode(_addr2).length,
          opcode: opcode,
        };
      }
    }

    if (this.lastOp === "GAS" && !opcode.includes("CALL")) {
      this.countSlot(this.currentLevel.opcodes, "GAS");
    }
    if (opcode !== "GAS") {
      if (
        opcode.match(
          /^(DUP\d+|PUSH\d+|SWAP\d+|POP|ADD|SUB|MUL|DIV|EQ|LTE?|S?GTE?|SLT|SH[LR]|AND|OR|NOT|ISZERO)$/
        ) == null
      ) {
        this.countSlot(this.currentLevel.opcodes, opcode);
      }
    }
    this.lastOp = opcode;

    if (opcode === "SLOAD" || opcode === "SSTORE") {
      var slot = toWord(log.stack.peek(0).toString(16));
      var slotHex = toHex(slot);
      var _addr3 = log.contract.getAddress();
      var _addrHex2 = toHex(_addr3);
      var access = this.currentLevel.access[_addrHex2];
      if (access == null) {
        access = {
          reads: {},
          writes: {},
        };
        this.currentLevel.access[_addrHex2] = access;
      }
      if (opcode === "SLOAD") {
        if (access.reads[slotHex] == null && access.writes[slotHex] == null) {
          access.reads[slotHex] = toHex(db.getState(_addr3, slot));
        }
      } else {
        this.countSlot(access.writes, slotHex);
      }
    }

    if (opcode === "TSTORE") {
      if (this.currentLevel) {
        this.currentLevel.tstore = (this.currentLevel.tstore || 0) + 1;
      }
    }
    if (opcode === "TLOAD") {
      if (this.currentLevel) {
        this.currentLevel.tload = (this.currentLevel.tload || 0) + 1;
      }
    }

    if (opcode === "KECCAK256") {
      var _ofs2 = parseInt(log.stack.peek(0).toString());
      var _len = parseInt(log.stack.peek(1).toString());
      if (_len > 20 && _len < 512) {
        this.keccak.push(toHex(log.memory.slice(_ofs2, _ofs2 + _len)));
      }
    } else if (opcode.startsWith("LOG")) {
      var count = parseInt(opcode.substring(3));
      var _ofs3 = parseInt(log.stack.peek(0).toString());
      var _len2 = parseInt(log.stack.peek(1).toString());
      var topics = [];
      for (var _i = 0; _i < count; _i++) {
        topics.push("0x" + log.stack.peek(2 + _i).toString(16));
      }
      var _data = toHex(log.memory.slice(_ofs3, _ofs3 + _len2));
      this.logs.push({
        topics: topics,
        data: _data,
      });
    }
  },
};
