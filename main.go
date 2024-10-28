package main

import (
	"log"
	"walletCount/src/services"
)

func main() {
	// Create instances
	client, err := services.NewEvmClient()
	if err != nil {
		log.Fatal("Error creating client instance", err)
	}

	// Count wallets in blockchain
	count, err := client.GetWalletAmount()
	if err != nil {
		log.Fatal("Error getting the wallet amount", err)
	}

	log.Printf("There are %d created in the blockchain", count)
}
