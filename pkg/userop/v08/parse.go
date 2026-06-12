package v08

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strings"
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

	if f.Kind() == reflect.String && t == reflect.TypeOf(big.Int{}) {
		n := new(big.Int)
		n, ok := n.SetString(data.(string), 0)
		if !ok {
			return nil, errors.New("bigInt conversion failed")
		}
		return n, nil
	}

	if f.Kind() == reflect.Float64 && t == reflect.TypeOf(big.Int{}) {
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

	return data, nil
}

// normalizeOptionalFields fills request fields that the ERC-4337 RPC spec
// marks optional (the factory and paymaster groups) so decoding and
// required-field validation treat an omitted group as empty. A dependent
// field without its parent (e.g. factoryData without factory) is rejected.
func normalizeOptionalFields(data map[string]any) (map[string]any, error) {
	zeroAddress := "0x0000000000000000000000000000000000000000"

	d := make(map[string]any, len(data)+6)
	for k, v := range data {
		// Treat JSON null the same as an omitted field.
		if v == nil {
			continue
		}
		d[k] = v
	}

	if _, ok := d["factory"]; !ok {
		if _, ok := d["factoryData"]; ok {
			return nil, errors.New("factoryData provided without factory")
		}
		d["factory"] = zeroAddress
		d["factoryData"] = "0x"
	} else if _, ok := d["factoryData"]; !ok {
		d["factoryData"] = "0x"
	}

	// The EIP-7702 marker factory "0x7702" denotes a delegated EOA sender. The
	// EntryPoint matches it left-aligned (initCode starting with bytes 0x7702),
	// so expand it before HexToAddress right-aligns it.
	if f, ok := d["factory"].(string); ok && strings.EqualFold(f, "0x7702") {
		d["factory"] = EIP7702MarkerFactory.Hex()
	}

	if _, ok := d["paymaster"]; !ok {
		for _, k := range []string{"paymasterVerificationGasLimit", "paymasterPostOpGasLimit", "paymasterData"} {
			if _, ok := d[k]; ok {
				return nil, fmt.Errorf("%s provided without paymaster", k)
			}
		}
		d["paymaster"] = zeroAddress
		d["paymasterVerificationGasLimit"] = "0x0"
		d["paymasterPostOpGasLimit"] = "0x0"
		d["paymasterData"] = "0x"
	} else {
		if _, ok := d["paymasterVerificationGasLimit"]; !ok {
			d["paymasterVerificationGasLimit"] = "0x0"
		}
		if _, ok := d["paymasterPostOpGasLimit"]; !ok {
			d["paymasterPostOpGasLimit"] = "0x0"
		}
		if _, ok := d["paymasterData"]; !ok {
			d["paymasterData"] = "0x"
		}
	}

	return d, nil
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

	data, err := normalizeOptionalFields(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadUserOperationData, err)
	}

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
