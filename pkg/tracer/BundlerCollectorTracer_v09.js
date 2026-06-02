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
  //
  // NOTE: The standard ERC-4337 v0.8/v0.9 EntryPoint signature is
  //   handleOps(PackedUserOperation[] calldata ops, address payable beneficiary)
  // i.e. only two parameters. There is no top-level eip7702Auth parameter;
  // the eip7702Auth is a tail field of each PackedUserOperation struct in
  // the ops array. As a result, the JS tracer cannot reliably detect auth
  // presence from the calldata alone — it would need to walk every op
  // and decode the PackedUserOperation tail, which is impractical here.
  //
  // The Go simulation pipeline decodes the eip7702Auth from the
  // PackedUserOperation data after the trace completes. This helper
  // therefore returns false for the standard 2-parameter handleOps.
  // A future change can introduce a deeper parser if the deployment
  // targets a custom EntryPoint that exposes auth at the top level.
  _hasEip7702Auth: function (_inputHex) {
    return false;
  },

  // Return true if the validatePaymasterUserOp(...) call's calldata
  // includes a non-empty paymasterSignature tail. The signature is a
  // field of the nested PackedUserOperation struct (v0.9+ layout).
  //
  // Calldata layout after the 4-byte selector:
  //   Word 0  (0   ..64 )  head[0] = userOp offset
  //   Word 1  (64  ..128)  head[1] = userOpHash
  //   Word 2  (128 ..192)  head[2] = missingAccountFunds
  //   Word 3  (192 ..256)  userOp.sender
  //   Word 4  (256 ..320)  userOp.nonce
  //   Word 5  (320 ..384)  userOp.initCode offset
  //   Word 6  (384 ..448)  userOp.callData offset
  //   Word 7  (448 ..512)  userOp.accountGasLimits
  //   Word 8  (512 ..576)  userOp.preVerificationGas
  //   Word 9  (576 ..640)  userOp.gasFees
  //   Word 10 (640 ..704)  userOp.paymasterAndData offset
  //   Word 11 (704 ..768)  userOp.signature offset
  //   Word 12 (768 ..832)  userOp.paymasterSignature offset (v0.9)
  //
  // The paymasterSignature offset is relative to the start of the
  // PackedUserOperation struct (i.e. relative to byte 96 of the payload).
  // We then read the length at (96 + offset) bytes from payload start.
  _hasPaymasterSignature: function (inputHex) {
    if (!inputHex || inputHex.length < 10) return false;
    if (inputHex.slice(0, 10) !== "0xc04a6310") return false;
    var payload = inputHex.slice(10);
    if (payload.length < 832) return false;
    var paymasterSigOffsetHex = payload.slice(768, 832);
    var paymasterSigOffset = parseInt(paymasterSigOffsetHex, 16);
    if (isNaN(paymasterSigOffset)) return false;
    var absoluteOffsetHexChar = (96 + paymasterSigOffset) * 2;
    if (payload.length < absoluteOffsetHexChar + 64) return false;
    var paymasterSigLengthHex = payload.slice(absoluteOffsetHexChar, absoluteOffsetHexChar + 64);
    var paymasterSigLength = parseInt(paymasterSigLengthHex, 16);
    if (isNaN(paymasterSigLength)) return false;
    return paymasterSigLength > 0;
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
