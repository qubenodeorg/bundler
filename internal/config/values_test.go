package config

import (
	"testing"
	"time"
)

// TestStoreConfigOpTTL verifies that an unset store TTL falls back to the
// in-process op TTL for shared backends, while an explicit value (including 0)
// is honored.
func TestStoreConfigOpTTL(t *testing.T) {
	maxOpTTL := 180 * time.Second

	t.Run("unset shared backend falls back to MaxOpTTL", func(t *testing.T) {
		v := &Values{MaxOpTTL: maxOpTTL, StoreBackend: "redis"}
		if got := v.StoreConfig().OpTTL; got != maxOpTTL {
			t.Fatalf("OpTTL = %v, want %v", got, maxOpTTL)
		}
	})

	t.Run("explicit zero disables TTL", func(t *testing.T) {
		v := &Values{MaxOpTTL: maxOpTTL, StoreBackend: "redis", StoreOpTTLExplicit: true}
		if got := v.StoreConfig().OpTTL; got != 0 {
			t.Fatalf("OpTTL = %v, want 0 (disabled)", got)
		}
	})

	t.Run("explicit value is honored", func(t *testing.T) {
		v := &Values{MaxOpTTL: maxOpTTL, StoreBackend: "redis", StoreOpTTL: 30 * time.Second, StoreOpTTLExplicit: true}
		if got := v.StoreConfig().OpTTL; got != 30*time.Second {
			t.Fatalf("OpTTL = %v, want 30s", got)
		}
	})

	t.Run("local backend has no fallback", func(t *testing.T) {
		v := &Values{MaxOpTTL: maxOpTTL, StoreBackend: "badger"}
		if got := v.StoreConfig().OpTTL; got != 0 {
			t.Fatalf("OpTTL = %v, want 0", got)
		}
	})
}
