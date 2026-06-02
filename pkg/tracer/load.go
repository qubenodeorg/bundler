// Package tracer provides custom tracing capabilities to comply with EIP-4337 specifications for
// forbidden opcodes. Tracers are version-aware and ship one set per supported ERC-4337 EntryPoint
// version (v0.6, v0.7, v0.8, v0.9).
package tracer

import (
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"github.com/zarbanio/bundler/pkg/userop"
)

//go:embed BundlerCollectorTracer.js
//go:embed BundlerCollectorTracer_v07.js
//go:embed BundlerCollectorTracer_v08.js
//go:embed BundlerCollectorTracer_v09.js
//go:embed BundlerExecutionTracer.js
//go:embed BundlerExecutionTracer_v07.js
//go:embed BundlerExecutionTracer_v08.js
//go:embed BundlerExecutionTracer_v09.js
var files embed.FS

var (
	commentRegex    = regexp.MustCompile("(?m)^.*//.*$[\r\n]+")
	whiteSpaceRegex = regexp.MustCompile(`\B\s+|\s+\B`)
	constInitStr    = "var tracer ="
	endLineChar     = ";"

	entryPointPlaceholder = "__ENTRY_POINT_ADDRESS_HEX__"
)

// parse takes the raw tracer from file and removes all non-essential code.
func parse(code string) string {
	m := commentRegex.ReplaceAllString(code, "")
	m = strings.TrimSpace(m)
	m = strings.TrimPrefix(m, constInitStr)
	m = strings.TrimSuffix(m, endLineChar)
	m = whiteSpaceRegex.ReplaceAllString(m, "")
	return m
}

// BundlerTracerSet is a single (collector, execution) pair for a given EntryPoint version.
type BundlerTracerSet struct {
	Collector string
	Execution string
}

// Tracers holds the per-version tracer strings. The v0.6 tracers are kept verbatim
// for backward compatibility; the v0.7 / v0.8 / v0.9 variants capture the new validation
// flow, EIP-7702 data and (v0.9 only) getCurrentUserOpHash() calls.
type Tracers struct {
	V06 BundlerTracerSet
	V07 BundlerTracerSet
	V08 BundlerTracerSet
	V09 BundlerTracerSet
}

// NewTracers reads all *Bundler*Tracer.js files and returns a collection of strings that
// can be passed to debug_traceCall as a custom tracer, keyed by EntryPoint version.
func NewTracers() (*Tracers, error) {
	t := &Tracers{}

	err := fs.WalkDir(files, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		body, err := fs.ReadFile(files, path)
		if err != nil {
			return err
		}
		code := parse(string(body))

		base := strings.TrimSuffix(path, ".js")
		switch base {
		case "BundlerCollectorTracer":
			t.V06.Collector = code
		case "BundlerCollectorTracer_v07":
			t.V07.Collector = code
		case "BundlerCollectorTracer_v08":
			t.V08.Collector = code
		case "BundlerCollectorTracer_v09":
			t.V09.Collector = code
		case "BundlerExecutionTracer":
			t.V06.Execution = code
		case "BundlerExecutionTracer_v07":
			t.V07.Execution = code
		case "BundlerExecutionTracer_v08":
			t.V08.Execution = code
		case "BundlerExecutionTracer_v09":
			t.V09.Execution = code
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return t, nil
}

// ForVersion returns the (collector, execution) tracer pair for the given EntryPoint version.
// For v0.9 the EntryPoint address is injected into the execution tracer so that
// getCurrentUserOpHash() STATICCALLs can be detected.
func (t *Tracers) ForVersion(v userop.Version, entryPoint string) (BundlerTracerSet, error) {
	switch v {
	case userop.V0_6:
		return t.V06, nil
	case userop.V0_7:
		return t.V07, nil
	case userop.V0_8:
		return t.V08, nil
	case userop.V0_9:
		set := t.V09
		if entryPoint != "" {
			set.Execution = strings.ReplaceAll(
				set.Execution,
				entryPointPlaceholder,
				strings.ToLower(entryPoint),
			)
			set.Collector = strings.ReplaceAll(
				set.Collector,
				entryPointPlaceholder,
				strings.ToLower(entryPoint),
			)
		}
		return set, nil
	default:
		return BundlerTracerSet{}, fmt.Errorf("tracer: unsupported version %q", v)
	}
}
