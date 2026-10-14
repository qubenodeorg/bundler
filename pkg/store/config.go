package store

import (
	"math/big"
	"strings"
	"time"
)

// Backend identifies a concrete storage implementation.
type Backend string

const (
	// BackendBadger is the embedded, node-local default for single-instance
	// deployments.
	BackendBadger Backend = "badger"
	// BackendRedis is a Redis (or wire-compatible) shared backend.
	BackendRedis Backend = "redis"
	// BackendValkey is accepted as an alias for BackendRedis. Valkey is
	// wire-compatible with Redis, so both map to the same implementation.
	BackendValkey Backend = "valkey"
	// BackendMemory is an ephemeral, process-local backend for tests and local
	// development. It is never shared between replicas.
	BackendMemory Backend = "memory"
)

// TLSConfig configures client-side TLS for network backends.
type TLSConfig struct {
	CAFile             string
	CertFile           string
	KeyFile            string
	InsecureSkipVerify bool
}

// Config is the transport-agnostic configuration accepted by the factory.
//
// ChainID and Namespace are prepended to every key so a single shared
// Redis/Valkey cluster (or a single BadgerDB directory) can safely serve
// multiple chains and multiple deployments. Leaving both empty preserves the
// legacy, un-namespaced key layout used by pre-existing BadgerDB data.
type Config struct {
	Backend   Backend
	Namespace string
	ChainID   *big.Int

	// Badger.
	DataDirectory string

	// Redis/Valkey.
	URL        string
	Addrs      []string
	MasterName string
	Username   string
	Password   string
	DB         int
	TLS        *TLSConfig

	PoolSize     int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	MaxRetries   int

	// OpTTL bounds how long a pending UserOperation may live in the store so a
	// crashed replica cannot leave state behind forever. Zero disables TTL.
	OpTTL time.Duration
}

// KeyPrefix returns the deployment/chain namespace in the form
// "namespace:chainID" (omitting empty components). It is empty when neither is
// set, in which case keys keep their legacy layout.
func (c Config) KeyPrefix() string {
	parts := make([]string, 0, 2)
	if c.Namespace != "" {
		parts = append(parts, c.Namespace)
	}
	if c.ChainID != nil {
		parts = append(parts, c.ChainID.String())
	}
	return strings.Join(parts, ":")
}

// Key builds a namespaced storage key from its colon-joined parts. With an
// empty prefix it returns the bare key, which keeps existing BadgerDB data
// readable.
func (c Config) Key(parts ...string) string {
	key := strings.Join(parts, ":")
	if prefix := c.KeyPrefix(); prefix != "" {
		return prefix + ":" + key
	}
	return key
}

// KeyPattern returns a glob pattern matching every key produced by Key for the
// given prefix parts, used by backends that support prefix deletion or scan.
func (c Config) KeyPattern(parts ...string) string {
	return c.Key(parts...) + "*"
}
