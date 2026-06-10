package tracer_test

import (
	"testing"

	"github.com/zarbanio/bundler/pkg/tracer"
)

func TestTracerModeSelection(t *testing.T) {
	for _, tc := range []struct {
		value  string
		native bool
	}{
		{"", true},
		{"erc7562Tracer", true},
		{"callTracer", true},
		{"js", false},
		{"JS", false},
		{" js ", false},
		{"Js\n", false},
	} {
		if got := tracer.IsNativeCollector(tc.value); got != tc.native {
			t.Errorf("IsNativeCollector(%q) = %v, want %v", tc.value, got, tc.native)
		}
		if got := tracer.IsNativeExecutor(tc.value); got != tc.native {
			t.Errorf("IsNativeExecutor(%q) = %v, want %v", tc.value, got, tc.native)
		}
	}
}
