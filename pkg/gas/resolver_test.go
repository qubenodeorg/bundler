package gas

import (
	"testing"

	"github.com/zarbanio/bundler/pkg/userop"
)

func TestOverheadResolver_ForReturnsMatchingVersion(t *testing.T) {
	r := NewOverheadResolver(userop.V0_7, userop.V0_8, userop.V0_9)

	cases := []userop.Version{userop.V0_6, userop.V0_7, userop.V0_8, userop.V0_9}
	for _, v := range cases {
		if got := r.For(v).GetVersion(); got != v {
			t.Fatalf("For(%s): got version %s", v, got)
		}
	}
}

func TestOverheadResolver_AlwaysIncludesV06Fallback(t *testing.T) {
	// Built without v0.6 explicitly; it must still be present as the fallback.
	r := NewOverheadResolver(userop.V0_9)

	if got := r.For(userop.V0_6).GetVersion(); got != userop.V0_6 {
		t.Fatalf("expected v0.6 fallback, got %s", got)
	}
	// An unknown version resolves to the v0.6 fallback rather than nil.
	if got := r.For(userop.Version("v9.9")).GetVersion(); got != userop.V0_6 {
		t.Fatalf("unknown version: expected v0.6 fallback, got %s", got)
	}
}

func TestOverheadResolver_EachVisitsEveryVersion(t *testing.T) {
	r := NewOverheadResolver(userop.V0_7, userop.V0_7, userop.V0_9) // duplicate collapsed

	seen := map[userop.Version]bool{}
	r.Each(func(v userop.Version, ov *Overhead) {
		if ov.GetVersion() != v {
			t.Fatalf("Each gave version %s for an Overhead reporting %s", v, ov.GetVersion())
		}
		seen[v] = true
	})

	for _, v := range []userop.Version{userop.V0_6, userop.V0_7, userop.V0_9} {
		if !seen[v] {
			t.Fatalf("Each did not visit %s", v)
		}
	}
	if seen[userop.V0_8] {
		t.Fatal("Each visited v0.8 which was never registered")
	}
}
