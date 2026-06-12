package reverts

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/zarbanio/bundler/pkg/errors"
)

// RPCErrorCodeForFailedOp maps an EntryPoint FailedOp reason (AAxx code) to
// the JSON-RPC error code defined by the ERC-4337 spec: signature failures
// (AA24 account, AA34 paymaster) to INVALID_SIGNATURE, other paymaster
// failures (AA3x) to REJECTED_BY_PAYMASTER, and everything else to
// REJECTED_BY_EP_OR_ACCOUNT.
func RPCErrorCodeForFailedOp(reason string) int {
	if strings.Contains(reason, "AA24") || strings.Contains(reason, "AA34") {
		return errors.INVALID_SIGNATURE
	}
	if strings.Contains(reason, "AA3") {
		return errors.REJECTED_BY_PAYMASTER
	}
	return errors.REJECTED_BY_EP_OR_ACCOUNT
}

// InnerRevertReason decodes a standard Error(string) revert payload, e.g. the
// inner bytes of a FailedOpWithRevert. It returns "" when the payload is
// empty or not an Error(string) encoding.
func InnerRevertReason(data []byte) string {
	reason, err := abi.UnpackRevert(data)
	if err != nil {
		return ""
	}
	return reason
}
