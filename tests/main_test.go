package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/userop"
)

var (
	paymasterUrl                = "http://localhost:4337"
	entryPointAddress           = common.HexToAddress("0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789")
	simpleAccountFactoryAddress = common.HexToAddress("0x9406Cc6185a346906296840746125a0E44976454")
)

func TestMain(t *testing.T) {
	t.Skip()
	client, err := ethclient.Dial("http://localhost:8545")
	if err != nil {
		log.Fatalf("Failed to connect to Ethereum client: %v", err)
	}

	privateKeyBytes, err := hexutil.Decode("0x29159aac2d04e9f28b3ea121ef9de1377a69e3e29ada32d7fd098c1a15b10bf5")
	if err != nil {
		log.Fatalf("Failed to decode private key: %v", err)
	}
	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		log.Fatalf("Failed to convert bytes to private key: %v", err)
	}
	ownerAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	fmt.Printf("Owner address: %s\n", ownerAddress.Hex())

	simpleAccountFactoryABI, err := abi.JSON(strings.NewReader(`[{"inputs":[{"internalType":"contract IEntryPoint","name":"_entryPoint","type":"address"}],"stateMutability":"nonpayable","type":"constructor"},{"inputs":[],"name":"accountImplementation","outputs":[{"internalType":"contract SimpleAccount","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"uint256","name":"salt","type":"uint256"}],"name":"createAccount","outputs":[{"internalType":"contract SimpleAccount","name":"ret","type":"address"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"uint256","name":"salt","type":"uint256"}],"name":"getAddress","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"}]`))
	if err != nil {
		log.Fatalf("Failed to parse ABI: %v", err)
	}

	nonce := big.NewInt(0)
	salt := big.NewInt(0)
	createAccountData, err := simpleAccountFactoryABI.Pack("createAccount", ownerAddress, salt)
	if err != nil {
		log.Fatalf("Failed to pack createAccount data: %v", err)
	}

	initCode := append(simpleAccountFactoryAddress.Bytes(), createAccountData...)

	// Gas limits (set appropriately)
	callGasLimit := big.NewInt(5_000_000)
	verificationGasLimit := big.NewInt(1_000_000)
	preVerificationGas := big.NewInt(50000)

	maxFeePerGas, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatalf("Failed to get gas price: %v", err)
	}
	maxPriorityFeePerGas := maxFeePerGas

	// Create the UserOperation without the signature first
	userOp := &userop.UserOperation{
		Sender:               common.HexToAddress("0x8c88d608ac044d7222774d78bb560ee47657b5e3"),
		Nonce:                nonce,
		InitCode:             initCode,
		CallGasLimit:         callGasLimit,
		VerificationGasLimit: verificationGasLimit,
		PreVerificationGas:   preVerificationGas,
		MaxFeePerGas:         maxFeePerGas,
		MaxPriorityFeePerGas: maxPriorityFeePerGas,
		CallData:             []byte{},
		PaymasterAndData:     []byte{},
		Signature:            []byte{}, // Placeholder
	}

	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		log.Fatalf("Failed to get chain ID: %v", err)
	}

	userOpHash := userOp.GetUserOpHash(entryPointAddress, chainID)
	fmt.Println("UserOperationHash:", userOpHash.Hex())
	messageHash := accounts.TextHash([]byte(userOpHash.Bytes()))
	fmt.Println("MessageHash:", common.Bytes2Hex(messageHash))

	signature, err := crypto.Sign(messageHash, privateKey)
	if err != nil {
		log.Fatalf("Failed to sign UserOperationHash: %v", err)
	}
	if signature[64] < 27 {
		signature[64] += 27
	}
	userOp.Signature = signature

	newUserOp, err := GetPaymasterAndData(userOp, entryPointAddress)
	if err != nil {
		log.Fatalf("Failed to get paymaster and data: %v", err)
	}
	userOpHash = newUserOp.GetUserOpHash(entryPointAddress, chainID)
	fmt.Println("NewUserOperationHash:", userOpHash.Hex())
	messageHash = accounts.TextHash([]byte(userOpHash.Bytes()))
	fmt.Println("NewMessageHash:", common.Bytes2Hex(messageHash))

	signature, err = crypto.Sign(messageHash, privateKey)
	if err != nil {
		log.Fatalf("Failed to sign UserOperationHash: %v", err)
	}
	if signature[64] < 27 {
		signature[64] += 27
	}
	newUserOp.Signature = signature

	userOpJSON, err := json.MarshalIndent(newUserOp, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal UserOperation: %v", err)
	}

	fmt.Println("UserOperation:")
	fmt.Println(string(userOpJSON))

	// Send the UserOperation to the bundler
	// For example, using an RPC client to the bundler's endpoint
	bundlerURL := "http://localhost:4337" // Replace with your bundler URL

	rpcClient, err := rpc.Dial(bundlerURL)
	if err != nil {
		log.Fatalf("Failed to connect to bundler: %v", err)
	}

	var result common.Hash
	err = rpcClient.Call(&result, "eth_sendUserOperation", newUserOp, entryPointAddress.Hex())
	if err != nil {
		log.Fatalf("Bundler error: %v", err)
	}

	fmt.Printf("Bundler response: %s\n", result.Hex())
}

// Define the response struct
type SponsorUserOperationResponse struct {
	PaymasterAndData     string `json:"paymasterAndData"`
	PreVerificationGas   string `json:"preVerificationGas"`
	VerificationGasLimit string `json:"verificationGasLimit"`
	CallGasLimit         string `json:"callGasLimit"`
}

// Update the function implementation
func GetPaymasterAndData(userOp *userop.UserOperation, entryPointAddress common.Address) (*userop.UserOperation, error) {
	userOpMap, err := userOp.ToMap()
	if err != nil {
		return nil, fmt.Errorf("failed to convert UserOperation to map: %v", err)
	}

	// Prepare the JSON-RPC request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "pm_sponsorUserOperation",
		"params": []interface{}{
			userOpMap,
			entryPointAddress.Hex(),
			map[string]string{
				"type": "payg",
			},
		},
	}

	// Convert the request to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON-RPC request: %v", err)
	}

	// Send the POST request
	resp, err := http.Post(paymasterUrl, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to call pm_sponsorUserOperation: %v", err)
	}
	defer resp.Body.Close()

	// Read and parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var response struct {
		JSONRPC string                       `json:"jsonrpc"`
		ID      int                          `json:"id"`
		Result  SponsorUserOperationResponse `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    string `json:"data,omitempty"`
		} `json:"error,omitempty"`
	}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC response: %v", err)
	}

	if response.Error != nil {
		return nil, fmt.Errorf("pm_sponsorUserOperation error: %s", response.Error.Message)
	}

	j, _ := json.MarshalIndent(response, "", "  ")
	fmt.Println("Response:", string(j))

	userOp.PaymasterAndData = common.Hex2Bytes(strings.Trim(response.Result.PaymasterAndData, "0x"))

	// Convert gas values from hex strings to big.Int and update userOp
	if response.Result.PreVerificationGas != "" {
		preVerificationGas, err := hexutil.DecodeBig(response.Result.PreVerificationGas)
		if err != nil {
			return nil, fmt.Errorf("failed to decode preVerificationGas: %v", err)
		}
		userOp.PreVerificationGas = preVerificationGas
	}

	if response.Result.VerificationGasLimit != "" {
		verificationGasLimit, err := hexutil.DecodeBig(response.Result.VerificationGasLimit)
		if err != nil {
			return nil, fmt.Errorf("failed to decode verificationGasLimit: %v", err)
		}
		userOp.VerificationGasLimit = verificationGasLimit
	}

	if response.Result.CallGasLimit != "" {
		callGasLimit, err := hexutil.DecodeBig(response.Result.CallGasLimit)
		if err != nil {
			return nil, fmt.Errorf("failed to decode callGasLimit: %v", err)
		}
		userOp.CallGasLimit = callGasLimit
	}

	return userOp, nil
}
