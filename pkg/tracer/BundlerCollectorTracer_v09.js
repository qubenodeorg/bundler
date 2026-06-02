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
  validatePaymasterUserOpSelector: "0x",

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

  result: function result(_ctx, _db) {
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
      this.currentLevel.oog = true;
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

        this.currentLevel = this.callsFromEntryPoint[this.topLevelCallCounter] =
          {
            topLevelMethodSig: topLevelMethodSig,
            topLevelTargetAddress: topLevelTargetAddress,
            access: {},
            opcodes: {},
            extCodeAccessInfo: {},
            contractSize: {},
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
