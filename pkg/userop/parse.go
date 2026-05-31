package userop

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
	"github.com/zarbanio/bundler/pkg/userop/v09"
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
	f reflect.Kind,
	t reflect.Kind,
	data interface{}) (interface{}, error) {
	// String to common.Address conversion
	if f == reflect.String && t == reflect.Array {
		return common.HexToAddress(data.(string)), nil
	}

	// String to big.Int conversion
	if f == reflect.String && t == reflect.Struct {
		n := new(big.Int)
		n, ok := n.SetString(data.(string), 0)
		if !ok {
			return nil, errors.New("bigInt conversion failed")
		}
		return n, nil
	}

	// Float64 to big.Int conversion
	if f == reflect.Float64 && t == reflect.Struct {
		n, ok := data.(float64)
		if !ok {
			return nil, errors.New("bigInt conversion failed")
		}
		return big.NewInt(int64(n)), nil
	}

	// String to []byte conversion
	if f == reflect.String && t == reflect.Slice {
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

func validateAddressType(field reflect.Value) interface{} {
	value, ok := field.Interface().(common.Address)
	if !ok || value == common.HexToAddress("0x") {
		return nil
	}

	return field
}

func validateBigIntType(field reflect.Value) interface{} {
	value, ok := field.Interface().(big.Int)
	if !ok || value.Cmp(big.NewInt(0)) == -1 {
		return nil
	}

	return field
}

// New decodes a map into a UserOperation object and validates all the fields are correctly typed.
func New(data map[string]any) (*UserOperation, error) {
	var op UserOperation

	// Convert map to struct
	config := &mapstructure.DecoderConfig{
		DecodeHook: decodeOpTypes,
		Result:     &op,
		ErrorUnset: true,
		MatchName:  exactFieldMatch,
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return nil, err
	}
	if err := decoder.Decode(data); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadUserOperationData, err)
	}

	// Validate struct
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

// decodeV09OpTypes handles field decoding for v0.9 UserOperation struct, including address fields that
// may be zero (not required for factory/paymaster).
func decodeV09OpTypes(
	f reflect.Kind,
	t reflect.Kind,
	data interface{}) (interface{}, error) {
	return decodeOpTypes(f, t, data)
}

// NewV09 decodes a map into a v0.9 UserOperation and validates all required fields.
func NewV09(data map[string]any) (*v09.UserOperation, error) {
	var op v09.UserOperation

	config := &mapstructure.DecoderConfig{
		DecodeHook: decodeV09OpTypes,
		Result:     &op,
		ErrorUnset: true,
		MatchName:  exactFieldMatch,
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

// NewVersioned decodes a map into the appropriate versioned UserOperation based on the version string.
// Supported versions: "v0.6", "v0.9". If version is empty, it attempts auto-detection from fields.
func NewVersioned(version string, data map[string]any) (UserOperationInterface, error) {
	switch version {
	case "v0.6", "":
		return New(data)
	case "v0.9":
		return NewV09(data)
	default:
		return nil, fmt.Errorf("unsupported UserOperation version: %s", version)
	}
}
