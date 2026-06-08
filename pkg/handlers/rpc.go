package handlers

type SponsorUserOperationResponse struct {
	PaymasterAndData     string `json:"paymasterAndData"`
	PaymasterSignature   string `json:"paymasterSignature,omitempty"`
	PreVerificationGas   string `json:"preVerificationGas"`
	VerificationGasLimit string `json:"verificationGasLimit"`
	CallGasLimit         string `json:"callGasLimit"`
}
