package main

import (
	"context"
	"log"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

var wg sync.WaitGroup

type EvmRepo struct {
	evmClient *ethclient.Client
}

type IBlock struct {
	Transactions types.Transactions `json:"transactions"`
	Withdrawals  types.Withdrawals  `json:"withdrawals"`
	Time         uint64             `json:"time"`
	BlockNumber  uint64             `json:"blockNumber"`
	GasUsed      uint64             `json:"gasUsed"`
}

type ITransaction struct {
	Transaction *types.Transaction `json:"transaction"`
	Sender      string             `json:"sender"`
	Success     bool               `json:"success"`
	Timestamp   uint64             `json:"timestamp"`
	BlockNumber int64              `json:"BlockNumber"`
}

type TxData struct {
	Transactions types.Transactions
	Timestamp    uint64
	BlockNumber  uint64
}

func NewEvmClient(clientURL string) (*EvmRepo, error) {
	client, err := ethclient.Dial(clientURL)
	if err != nil {
		log.Fatal("Error getting the RPC connection", err)
		return nil, err
	}

	return &EvmRepo{
		evmClient: client,
	}, nil
}

func (evmClient EvmRepo) GetBlocks(startBlock int, endBlock int) ([]IBlock, error) {
	blocks := make(chan *types.Block, endBlock-startBlock+1)
	workers := make(chan bool, 50)
	for i := startBlock; i < endBlock; i++ {
		log.Println("Block", i)
		wg.Add(1)
		workers <- true
		blockNumber := i
		go func() {
			defer wg.Done()
			defer func() { <-workers }()
			block, err := evmClient.evmClient.BlockByNumber(context.Background(), big.NewInt(int64(blockNumber)))
			if err != nil {
				log.Println("Error getting the block from Blocknumber: ", err)
				return
			}
			blocks <- block
		}()
	}

	go func() {
		wg.Wait()
		close(blocks)
	}()

	var blocksArray []IBlock
	for block := range blocks {
		response := mapTransferResponse(block)
		blocksArray = append(blocksArray, response)
	}

	return blocksArray, nil
}

func (evmClient EvmRepo) fetchAllTransactions(blocks []IBlock) ([]ITransaction, error) {
	transactions := []ITransaction{}
	for index, block := range blocks {
		log.Printf("Transactions in block: %d of %d\n", index, len(blocks))
		if block.Transactions.Len() != 0 {
			transactionsData := TxData{
				Transactions: block.Transactions,
				Timestamp:    block.Time,
				BlockNumber:  block.BlockNumber,
			}
			receipts := evmClient.GetReceiptFromTransactions(transactionsData)
			transactions = append(transactions, receipts...)
		}
	}
	return transactions, nil
}

func (evmClient EvmRepo) GetReceiptFromTransactions(transactions TxData) []ITransaction {
	response := []ITransaction{}
	receipts := make(chan ITransaction, len(transactions.Transactions))
	workers := make(chan bool, 5)

	for _, transaction := range transactions.Transactions {
		wg.Add(1)
		workers <- true
		go func(entryTx *types.Transaction) {
			defer wg.Done()
			defer func() { <-workers }()
			tx, _, err := evmClient.getTransactionByHash(context.Background(), entryTx.Hash())
			if err != nil {
				return
			}

			receipt := ITransaction{}
			receipt.Success = true

			signer := types.LatestSignerForChainID(tx.ChainId())
			from, err := types.Sender(signer, tx)
			if err != nil {
				log.Println("Error getting the sender: ", err)
				receipt.Success = false
			}

			receipt.Transaction = tx
			receipt.Sender = from.String()
			receipt.Timestamp = transactions.Timestamp
			receipt.BlockNumber = int64(transactions.BlockNumber)
			receipts <- receipt
		}(transaction)
	}

	go func() {
		wg.Wait()
		close(receipts)
		close(workers)
	}()

	for receipt := range receipts {
		response = append(response, receipt)
	}
	return response
}

func mapTransferResponse(block *types.Block) IBlock {
	convertMstoS := 1000
	return IBlock{
		Transactions: block.Transactions(),
		Withdrawals:  block.Withdrawals(),
		Time:         block.Time() * uint64(convertMstoS),
		BlockNumber:  block.NumberU64(),
		GasUsed:      block.GasUsed(),
	}
}

func (evmClient EvmRepo) getTransactionByHash(context context.Context, hash common.Hash) (*types.Transaction, bool, error) {
	tx, pending, err := evmClient.evmClient.TransactionByHash(context, hash)
	if err != nil {
		log.Println("Error getting the transaction receipt", err)
		return nil, false, err
	}
	return tx, pending, nil
}

func main() {
	evmClient, err := NewEvmClient("https://a.sentry.testnet.kiivalidator.com:8645/")
	if err != nil {
		log.Fatal("RPC Connection fails ", err)
	}

	blocks, err := evmClient.GetBlocks(1, 500000)
	if err != nil {
		log.Fatal("Error getting the blocks", err)
	}

	txs, err := evmClient.fetchAllTransactions(blocks)
	if err != nil {
		log.Fatal("Error getting the transaction from blocks", err)
	}

	wallets := make(map[string]bool)
	count := 0

	for _, tx := range txs {
		if tx.Transaction.To() != nil {
			to := strings.ToLower(tx.Transaction.To().Hex())
			from := strings.ToLower(tx.Sender)

			if !wallets[to] {
				wallets[to] = true
				count++
				log.Println("Wallet count: ", count)
			}

			if !wallets[from] {
				wallets[from] = true
				count++
				log.Println("Wallet count: ", count)
			}
		}
	}
	log.Print("Wallet amount:", count)
}
