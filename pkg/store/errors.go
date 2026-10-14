// Package store defines the pluggable persistence interfaces used by the
// bundler's mempool, reputation tracker, and code-hash cache.
//
// The interfaces are intentionally narrow and context-aware so they can be
// backed by a node-local embedded database (BadgerDB), a process-local map
// (memory), or a shared network store (Redis/Valkey) without callers knowing
// which one is in use.
package store

import "errors"

var (
	// ErrNotFound is returned by read operations when the requested key does
	// not exist. Callers can distinguish "absent" from a backend failure with
	// errors.Is(err, ErrNotFound).
	ErrNotFound = errors.New("store: key not found")

	// ErrUnavailable is returned when the backend cannot be reached or has
	// failed. Readiness probes and callers should treat it as a transient
	// dependency failure rather than a missing record.
	ErrUnavailable = errors.New("store: backend unavailable")
)

// IsNotFound reports whether err, or any error it wraps, is ErrNotFound.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsUnavailable reports whether err, or any error it wraps, is ErrUnavailable.
func IsUnavailable(err error) bool { return errors.Is(err, ErrUnavailable) }
