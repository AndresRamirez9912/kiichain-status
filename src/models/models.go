package models

type TransactionsByPageResponse struct {
	BasicResponse
	Transactions []ITransaction `json:"transactions"`
	Quantity     int            `json:"quantity"`
	Page         int            `json:"page"`
}

type BasicResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"errorMessage"`
}

type ITransaction struct {
	Transaction IReceipt `json:"transaction"`
	Sender      string   `json:"sender"`
	Success     bool     `json:"success"`
	Timestamp   int      `json:"timestamp"`
	BlockNumber int      `json:"BlockNumber"`
}

type IReceipt struct {
	ChainId  string
	To       string
	Gas      string
	GasPrice string
	Hash     string
}
