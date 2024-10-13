package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
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

func TestMain(t *testing.T) {
	// Initialize Ethereum client
	client, err := ethclient.Dial("http://localhost:8545")
	if err != nil {
		log.Fatalf("Failed to connect to Ethereum client: %v", err)
	}

	// EntryPoint contract address (replace with the correct address)
	entryPointAddress := common.HexToAddress("0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789")

	// SimpleAccountFactory contract address (replace with the correct address)
	simpleAccountFactoryAddress := common.HexToAddress("0x9406Cc6185a346906296840746125a0E44976454")

	// ABI of the SimpleAccountFactory
	simpleAccountFactoryABI, err := abi.JSON(strings.NewReader(`[{"inputs":[{"internalType":"contract IEntryPoint","name":"_entryPoint","type":"address"}],"stateMutability":"nonpayable","type":"constructor"},{"inputs":[],"name":"accountImplementation","outputs":[{"internalType":"contract SimpleAccount","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"uint256","name":"salt","type":"uint256"}],"name":"createAccount","outputs":[{"internalType":"contract SimpleAccount","name":"ret","type":"address"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"uint256","name":"salt","type":"uint256"}],"name":"getAddress","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"}]`))
	if err != nil {
		log.Fatalf("Failed to parse ABI: %v", err)
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

	// Nonce is typically 0 for a new account
	nonce := big.NewInt(0)

	// Use a salt value for CREATE2 deployment; here we use 0
	salt := big.NewInt(0)

	// Encode the function call to createAccount(owner, salt)
	createAccountData, err := simpleAccountFactoryABI.Pack("createAccount", ownerAddress, salt)
	if err != nil {
		log.Fatalf("Failed to pack createAccount data: %v", err)
	}

	// Construct initCode: factory address + encoded function call
	initCode := append(simpleAccountFactoryAddress.Bytes(), createAccountData...)

	// For this example, we will not send any callData
	callData := []byte{}

	// Gas limits (set appropriately)
	callGasLimit := big.NewInt(5_000_000)
	verificationGasLimit := big.NewInt(1_000_000)
	preVerificationGas := big.NewInt(50000)

	// Gas prices (set appropriately or fetch from network)
	maxFeePerGas, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatalf("Failed to get gas price: %v", err)
	}
	maxPriorityFeePerGas := maxFeePerGas

	// No paymaster in this example
	paymasterAndData := []byte{}

	// Create the UserOperation without the signature first
	userOp := &userop.UserOperation{
		Sender:               common.HexToAddress("0x8c88d608ac044d7222774d78bb560ee47657b5e3"),
		Nonce:                nonce,
		InitCode:             initCode,
		CallData:             callData,
		CallGasLimit:         callGasLimit,
		VerificationGasLimit: verificationGasLimit,
		PreVerificationGas:   preVerificationGas,
		MaxFeePerGas:         maxFeePerGas,
		MaxPriorityFeePerGas: maxPriorityFeePerGas,
		PaymasterAndData:     paymasterAndData,
		Signature:            []byte{}, // Placeholder
	}

	// Compute the UserOperationHash
	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		log.Fatalf("Failed to get chain ID: %v", err)
	}

	userOpHash := userOp.GetUserOpHash(entryPointAddress, chainID)

	fmt.Println("UserOperationHash:", userOpHash.Hex())
	messageHash := accounts.TextHash([]byte(userOpHash.Bytes()))
	fmt.Println("MessageHash:", common.Bytes2Hex(messageHash))
	// Sign the UserOperationHash with the owner's private key
	signature, err := crypto.Sign(messageHash, privateKey)
	if err != nil {
		log.Fatalf("Failed to sign UserOperationHash: %v", err)
	}
	signature[64] += 27

	// Update the UserOperation with the signature
	userOp.Signature = signature

	// The UserOperation is now ready to be sent to the bundler
	// Convert the UserOperation to JSON as required by the bundler's API
	userOpJSON, err := json.MarshalIndent(userOp, "", "  ")
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
	err = rpcClient.Call(&result, "eth_sendUserOperation", userOp, entryPointAddress.Hex())
	if err != nil {
		log.Fatalf("Bundler error: %v", err)
	}

	fmt.Printf("Bundler response: %s\n", result.Hex())
}

func BigIntToBytes32(i *big.Int) [32]byte {
	return [32]byte(common.LeftPadBytes(i.Bytes(), 32))
}
