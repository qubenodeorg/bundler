package checks

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dgraph-io/badger/v3"
	"github.com/ethereum/go-ethereum/common"
)

func newMemDB(t *testing.T) *badger.DB {
	t.Helper()
	db, err := badger.Open(badger.DefaultOptions("").WithInMemory(true).WithLoggingLevel(badger.ERROR))
	if err != nil {
		t.Fatalf("badger.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestGetCodeHashesKey_StableEncoding(t *testing.T) {
	h := common.HexToHash("0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	key := string(getCodeHashesKey(h))
	if !strings.Contains(key, h.String()) {
		t.Fatalf("key %q should embed userOpHash %s", key, h.String())
	}
	if !strings.HasPrefix(key, string(codeHashesPrefix)) {
		t.Fatalf("key %q should start with codeHashesPrefix %s", key, string(codeHashesPrefix))
	}
}

func TestGetSavedCodeHashes_RoundTrip(t *testing.T) {
	db := newMemDB(t)
	userOpHash := common.HexToHash("0x01")
	want := []codeHash{
		{Address: common.HexToAddress("0xaa"), Hash: common.HexToHash("0xbb")},
		{Address: common.HexToAddress("0xcc"), Hash: common.HexToHash("0xdd")},
	}
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := db.Update(func(txn *badger.Txn) error {
		return txn.Set(getCodeHashesKey(userOpHash), raw)
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := getSavedCodeHashes(db, userOpHash)
	if err != nil {
		t.Fatalf("getSavedCodeHashes: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d hashes, got %d", len(want), len(got))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("[%d] got %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestGetSavedCodeHashes_NotFound(t *testing.T) {
	db := newMemDB(t)
	if _, err := getSavedCodeHashes(db, common.HexToHash("0x99")); err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestRemoveSavedCodeHashes(t *testing.T) {
	db := newMemDB(t)
	a := common.HexToHash("0x01")
	b := common.HexToHash("0x02")
	raw, err := json.Marshal([]codeHash{{Address: common.HexToAddress("0xaa")}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(getCodeHashesKey(a), raw); err != nil {
			return err
		}
		return txn.Set(getCodeHashesKey(b), raw)
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := removeSavedCodeHashes(db, a, b); err != nil {
		t.Fatalf("removeSavedCodeHashes: %v", err)
	}

	if _, err := getSavedCodeHashes(db, a); err == nil {
		t.Fatal("expected hash a to be removed")
	}
	if _, err := getSavedCodeHashes(db, b); err == nil {
		t.Fatal("expected hash b to be removed")
	}
}
