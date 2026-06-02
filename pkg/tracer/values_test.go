package tracer

import (
	"testing"

	"github.com/zarbanio/bundler/pkg/userop"
)

func TestLoaded_PopulatedAtInit(t *testing.T) {
	if Loaded == nil {
		t.Fatal("Loaded should be initialized at package init")
	}
	if Loaded.V06.Collector == "" || Loaded.V06.Execution == "" {
		t.Error("Loaded.V06 should be populated")
	}
	if Loaded.V07.Collector == "" || Loaded.V07.Execution == "" {
		t.Error("Loaded.V07 should be populated")
	}
	if Loaded.V08.Collector == "" || Loaded.V08.Execution == "" {
		t.Error("Loaded.V08 should be populated")
	}
	if Loaded.V09.Collector == "" || Loaded.V09.Execution == "" {
		t.Error("Loaded.V09 should be populated")
	}
}

func TestLoaded_HasAllKnownVersions(t *testing.T) {
	for _, v := range []userop.Version{userop.V0_6, userop.V0_7, userop.V0_8, userop.V0_9} {
		set, err := Loaded.ForVersion(v, "")
		if err != nil {
			t.Errorf("Loaded.ForVersion(%s): %v", v, err)
			continue
		}
		if set.Collector == "" || set.Execution == "" {
			t.Errorf("Loaded.ForVersion(%s) returned empty tracer set", v)
		}
	}
}

func TestLoaded_ForVersion_MatchesNewTracers(t *testing.T) {
	ts, err := NewTracers()
	if err != nil {
		t.Fatalf("NewTracers: %v", err)
	}
	for _, v := range []userop.Version{userop.V0_6, userop.V0_7, userop.V0_8, userop.V0_9} {
		got, err := Loaded.ForVersion(v, "")
		if err != nil {
			t.Fatalf("Loaded.ForVersion(%s): %v", v, err)
		}
		want, err := ts.ForVersion(v, "")
		if err != nil {
			t.Fatalf("ts.ForVersion(%s): %v", v, err)
		}
		if got.Collector != want.Collector {
			t.Errorf("%s collector differs between Loaded and NewTracers", v)
		}
		if got.Execution != want.Execution {
			t.Errorf("%s execution differs between Loaded and NewTracers", v)
		}
	}
}
