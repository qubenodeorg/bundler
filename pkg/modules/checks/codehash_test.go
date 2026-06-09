package checks

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func makeGetCode(codes map[common.Address][]byte, errOnAddr *common.Address) GetCodeFunc {
	return func(addr common.Address) ([]byte, error) {
		if errOnAddr != nil && *errOnAddr == addr {
			return nil, errors.New("rpc error")
		}
		return codes[addr], nil
	}
}

func TestGetCodeHashes_Empty(t *testing.T) {
	got, err := getCodeHashes(nil, func(addr common.Address) ([]byte, error) {
		t.Fatalf("should not be called")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result")
	}
}

func TestGetCodeHashes_Computed(t *testing.T) {
	a := common.HexToAddress("0xaa")
	b := common.HexToAddress("0xbb")
	codes := map[common.Address][]byte{
		a: {0x01, 0x02},
		b: {0x03, 0x04},
	}
	got, err := getCodeHashes([]common.Address{a, b}, makeGetCode(codes, nil))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries")
	}
	if got[0].Hash != crypto.Keccak256Hash(codes[a]) {
		t.Fatalf("hash[a] mismatch")
	}
	if got[1].Hash != crypto.Keccak256Hash(codes[b]) {
		t.Fatalf("hash[b] mismatch")
	}
}

func TestGetCodeHashes_Error(t *testing.T) {
	a := common.HexToAddress("0xaa")
	_, err := getCodeHashes([]common.Address{a}, makeGetCode(nil, &a))
	if err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestHasCodeHashChanges_NoChange(t *testing.T) {
	a := common.HexToAddress("0xaa")
	bytecode := []byte{0x01, 0x02}
	hash := crypto.Keccak256Hash(bytecode)
	chs := []codeHash{{Address: a, Hash: hash}}
	codes := map[common.Address][]byte{a: bytecode}
	changed, err := hasCodeHashChanges(chs, makeGetCode(codes, nil))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if changed {
		t.Fatal("expected no changes")
	}
}

func TestHasCodeHashChanges_Detected(t *testing.T) {
	a := common.HexToAddress("0xaa")
	chs := []codeHash{{Address: a, Hash: crypto.Keccak256Hash([]byte{0x01})}}
	codes := map[common.Address][]byte{a: {0x02}}
	changed, err := hasCodeHashChanges(chs, makeGetCode(codes, nil))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !changed {
		t.Fatal("expected changes")
	}
}

func TestHasCodeHashChanges_Error(t *testing.T) {
	a := common.HexToAddress("0xaa")
	chs := []codeHash{{Address: a}}
	if _, err := hasCodeHashChanges(chs, makeGetCode(nil, &a)); err == nil {
		t.Fatal("expected error")
	}
}
