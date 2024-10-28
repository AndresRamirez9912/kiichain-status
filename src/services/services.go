package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"walletCount/src/models"
)

type EvmClient struct {
	client        http.Client
	currentTxPage int
	url           string
}

func NewEvmClient() (*EvmClient, error) {
	// Get the current page in the backend
	url := "https://kii.backend.kiivalidator.com/explorer/transactions"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	jsonData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	response := &models.TransactionsByPageResponse{}
	err = json.Unmarshal(jsonData, response)
	if err != nil {
		return nil, err
	}

	return &EvmClient{
		client:        http.Client{},
		currentTxPage: response.Page,
		url:           url,
	}, nil
}

func (evm *EvmClient) GetTxInPage(page int) ([]models.ITransaction, error) {
	// Create Request
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/%d", evm.url, page), nil)
	if err != nil {
		log.Println("Error creating the request", err)
		return nil, err
	}

	// Send Request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error making the rest request", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Receive Response
	jsonData, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error getting the response data", err)
		return nil, err
	}

	response := &models.TransactionsByPageResponse{}
	err = json.Unmarshal(jsonData, response)
	if err != nil {
		log.Println("Error unmarshaling the received data", err)
		return nil, err
	}

	// Validate the received response
	if response.Quantity == 0 {
		log.Println("Response information not valid")
		return nil, errors.New("Response information not valid")
	}

	return response.Transactions, nil
}

func (evm *EvmClient) GetWalletAmount() (int, error) {
	wallets := make(map[string]bool)
	walletsCount := 0
	var mutex sync.Mutex // Mutex for protecting the map access

	// Get the blocks and their transactions
	var wg sync.WaitGroup
	workers := make(chan bool, 5)                                 // Canal para limitar las goroutines concurrentes
	txChan := make(chan []models.ITransaction, evm.currentTxPage) // channel to receive the transactions

	// throw goroutines
	for i := evm.currentTxPage; i > 0; i-- {
		wg.Add(1)
		workers <- true
		go func(page int) {
			defer func() {
				wg.Done()
				<-workers
			}()
			log.Printf("Page requested: %d", page)
			transactions, err := evm.GetTxInPage(page)
			if err != nil {
				return
			}
			txChan <- transactions
		}(i)
	}

	// close channels after the getting transaction process
	go func() {
		wg.Wait()
		close(txChan)
		close(workers)
	}()

	// process the transactions from the chan
	for transactions := range txChan {
		for _, transaction := range transactions {
			sender := transaction.Sender
			to := transaction.Transaction.To

			// protect the access of the wallets map
			mutex.Lock()
			if !wallets[sender] {
				wallets[sender] = true
				walletsCount++
			}

			if to != "" && !wallets[to] {
				wallets[to] = true
				walletsCount++
			}
			mutex.Unlock()
		}
	}

	return walletsCount, nil
}
