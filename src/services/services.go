package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"walletCount/src/models"

	"github.com/redis/go-redis/v9"
)

type EvmClient struct {
	currentTxPage int
	redisClient   *redis.Client
}

func NewEvmClient() (*EvmClient, error) {
	// Create Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	// Ping Redis
	_, err := redisClient.Ping(context.Background()).Result()
	if err != nil {
		log.Fatal("Error connecting to the Redis server", err)
		return nil, err
	}

	data, err := redisClient.Get(context.Background(), "cachedPages").Result()
	if err != nil {
		log.Println("Error getting the cached pages in redis", err.Error())
		return nil, err
	}

	pages := []int{}
	err = json.Unmarshal([]byte(data), &pages)
	if err != nil {
		log.Println("Error decrypting the cached pages in redis", err.Error())
		return nil, err
	}

	return &EvmClient{
		currentTxPage: pages[len(pages)-1],
		redisClient:   redisClient,
	}, nil
}

func (evm *EvmClient) GetTxInPage(page int) ([]models.ITransaction, error) {
	// Get the information per page
	transactions := []models.ITransaction{}
	value, err := evm.redisClient.Get(context.Background(), "transaction:"+fmt.Sprint(page)).Result()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(value), &transactions)
	if err != nil {
		log.Println("Error decoding the transactions in redis", err)
		return nil, err
	}

	// Validate the received response
	if len(transactions) == 0 {
		log.Println("Response information not valid")
		return nil, errors.New("Response information not valid")
	}

	return transactions, nil
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
