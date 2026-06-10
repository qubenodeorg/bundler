package tracer

import (
	"encoding/json"
	"strings"
)

// Tracer selection values for the erc4337_bundler_native_bundler_*_tracer env
// vars. An empty value defaults to the native tracer; set "js" to fall back to
// the embedded JS tracers (only for nodes without native tracer support — the
// JS path runs on a per-opcode interpreter inside the node and is far slower).
const (
	// NativeCollector is the node's built-in ERC-7562 validation tracer.
	NativeCollector = "erc7562Tracer"
	// NativeExecutor is the node's built-in call tracer (run with
	// tracerConfig {"withLog": true}).
	NativeExecutor = "callTracer"
	// MuxTracer runs multiple native tracers in a single debug_traceCall.
	MuxTracer = "muxTracer"
	// JSFallback selects the embedded JS tracers.
	JSFallback = "js"
)

// IsNativeCollector reports whether the configured collector tracer value
// selects the native erc7562Tracer. Anything other than an explicit "js"
// selects native.
func IsNativeCollector(v string) bool {
	return !strings.EqualFold(strings.TrimSpace(v), JSFallback)
}

// IsNativeExecutor reports whether the configured executor tracer value
// selects the native callTracer. Anything other than an explicit "js"
// selects native.
func IsNativeExecutor(v string) bool {
	return !strings.EqualFold(strings.TrimSpace(v), JSFallback)
}

// CollectorName returns the effective collector tracer name for logging.
func CollectorName(v string) string {
	if IsNativeCollector(v) {
		return NativeCollector
	}
	return JSFallback
}

// ExecutorName returns the effective executor tracer name for logging.
func ExecutorName(v string) string {
	if IsNativeExecutor(v) {
		return NativeExecutor
	}
	return JSFallback
}

// CallTracerConfig is the tracerConfig payload for the native callTracer.
func CallTracerConfig() map[string]any {
	return map[string]any{"withLog": true}
}

// Erc7562TracerConfig is the tracerConfig payload for the native erc7562Tracer.
func Erc7562TracerConfig() map[string]any {
	return map[string]any{"withLog": true}
}

// MuxTracerConfig is the tracerConfig payload to run the erc7562Tracer and
// callTracer in one debug_traceCall round-trip (one EVM execution).
func MuxTracerConfig() map[string]any {
	return map[string]any{
		NativeCollector: Erc7562TracerConfig(),
		NativeExecutor:  CallTracerConfig(),
	}
}

// MuxedTrace is the response envelope of a muxTracer call combining the two
// native tracers. CallTracer is kept raw because the callTracer frame type
// lives in pkg/entrypoint/execution.
type MuxedTrace struct {
	Erc7562    *Erc7562Frame   `json:"erc7562Tracer"`
	CallTracer json.RawMessage `json:"callTracer"`
}
