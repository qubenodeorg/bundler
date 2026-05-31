package v08

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-playground/validator/v10"
	"github.com/mitchellh/mapstructure"
)

var (
	validate = validator.New()
	onlyOnce = sync.Once{}

	ErrBadUserOperationData = errors.New("cannot decode UserOperation")
)

func exactFieldMatch(mapKey, fieldName string) bool {
	return mapKey == fieldName
}

func decodeOpTypes(
	f reflect.Type,
	t reflect.Type,
	data interface{}) (interface{}, error) {
	if f.Kind() == reflect.String && t.Kind() == reflect.Array {
		return common.HexToAddress(data.(string)), nil
	}

	if f.Kind() == reflect.String && t.Kind() == reflect.Struct {
		n := new(big.Int)
		n, ok := n.SetString(data.(string), 0)
		if !ok {
			return nil, errors.New("bigInt conversion failed")
		}
		return n, nil
	}

	if f.Kind() == reflect.Float64 && t.Kind() == reflect.Struct {
		n, ok := data.(float64)
		if !ok {
			return nil, errors.New("bigInt conversion failed")
		}
		return big.NewInt(int64(n)), nil
	}

	if f.Kind() == reflect.String && t.Kind() == reflect.Slice {
		byteStr := data.(string)
		if len(byteStr) < 2 || byteStr[:2] != "0x" {
			return nil, errors.New("not byte string")
		}

		b, err := hex.DecodeString(byteStr[2:])
		if err != nil {
			return nil, err
		}
		return b, nil
	}

	if f.Kind() == reflect.Map && t == reflect.TypeOf(&Authorization{}) {
		// Decode EIP7702Auth map into Authorization struct
		m, ok := data.(map[string]interface{})
		if !ok {
			return data, nil
		}
		auth := &Authorization{}
		if chainId, ok := m["chainId"].(string); ok {
			n := new(big.Int)
			auth.ChainID, _ = n.SetString(chainId, 0)
		}
		if addr, ok := m["address"].(string); ok {
			auth.Address = common.HexToAddress(addr)
		}
		if nonce, ok := m["nonce"].(string); ok {
			n := new(big.Int)
			auth.Nonce, _ = n.SetString(nonce, 0)
		}
		if yParity, ok := m["yParity"].(float64); ok {
			auth.YParity = uint8(yParity)
		}
		if r, ok := m["r"].(string); ok {
			n := new(big.Int)
			auth.R, _ = n.SetString(r, 0)
		}
		if s, ok := m["s"].(string); ok {
			n := new(big.Int)
			auth.S, _ = n.SetString(s, 0)
		}
		return auth, nil
	}

	return data, nil
}

func validateAddressType(field reflect.Value) interface{} {
	value, ok := field.Interface().(common.Address)
	if !ok {
		return nil
	}
	return value.String()
}

func validateBigIntType(field reflect.Value) interface{} {
	value, ok := field.Interface().(big.Int)
	if !ok || value.Cmp(big.NewInt(0)) < 0 {
		return nil
	}
	return field
}

// New decodes a map into a v0.8 UserOperation object and validates all the fields are correctly typed.
func New(data map[string]any) (*UserOperation, error) {
	var op UserOperation

	config := &mapstructure.DecoderConfig{
		DecodeHook:       decodeOpTypes,
		Result:           &op,
		MatchName:        exactFieldMatch,
		WeaklyTypedInput: true,
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return nil, err
	}
	if err := decoder.Decode(data); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadUserOperationData, err)
	}

	onlyOnce.Do(func() {
		validate.RegisterCustomTypeFunc(validateAddressType, common.Address{})
		validate.RegisterCustomTypeFunc(validateBigIntType, big.Int{})
	})
	err = validate.Struct(op)
	if err != nil {
		return nil, err
	}

	return &op, nil
}
