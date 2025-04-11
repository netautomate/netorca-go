package main

import (
	"log"
	"time"

	"github.com/netautomate/netorca-go/config"
	"github.com/netautomate/netorca-go/pkg/client"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig(".env")
	baseURL := cfg.BaseURL
	apiKey := cfg.APIKey
	apiVer := cfg.APIVersion

	_, err := client.NewClient(baseURL, apiKey, apiVer, time.Duration(cfg.RequestTimeout)*time.Second)
	if err != nil {
		log.Fatalf("Failed to initialize SDK client: %v", err)
	}
	log.Println("SDK client initialized successfully")
}
