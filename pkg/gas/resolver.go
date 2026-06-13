package gas

import "github.com/zarbanio/bundler/pkg/userop"

// OverheadResolver holds one Overhead per ERC-4337 version so a single bundler
// instance can compute version-correct gas (PVG model, unused-gas penalty) for
// UserOperations targeting different EntryPoints at the same time.
//
// Each version's Overhead is independently configurable (e.g. rollup-specific
// preVerificationGas functions) via Each before the resolver is used.
type OverheadResolver struct {
	byVersion map[userop.Version]*Overhead
}

// NewOverheadResolver builds a resolver containing the default Overhead for each
// of the given versions. Duplicates are collapsed. v0.6 is always present so
// For never returns nil for the fallback version.
func NewOverheadResolver(versions ...userop.Version) *OverheadResolver {
	r := &OverheadResolver{byVersion: map[userop.Version]*Overhead{}}
	r.byVersion[userop.V0_6] = NewDefaultOverheadForVersion(userop.V0_6)
	for _, v := range versions {
		if _, ok := r.byVersion[v]; ok {
			continue
		}
		r.byVersion[v] = NewDefaultOverheadForVersion(v)
	}
	return r
}

// For returns the Overhead matching the given version, falling back to the v0.6
// default for any version the resolver was not built with.
func (r *OverheadResolver) For(v userop.Version) *Overhead {
	if ov, ok := r.byVersion[v]; ok {
		return ov
	}
	return r.byVersion[userop.V0_6]
}

// Each invokes fn for every (version, Overhead) pair held by the resolver. Use
// it to apply per-version configuration such as rollup preVerificationGas
// functions or buffer factors at startup.
func (r *OverheadResolver) Each(fn func(version userop.Version, ov *Overhead)) {
	for v, ov := range r.byVersion {
		fn(v, ov)
	}
}
