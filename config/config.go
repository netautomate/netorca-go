package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds the configuration for the API client.
type Config struct {
	// APIKey is the API key used for authentication.
	APIKey string
	// BaseURL is the base URL of the API.
	BaseURL string
	// APIVersion is the version of the API to use - by default use v1.
	APIVersion string
	// RequestTimeout is the timeout for API requests (in seconds).
	RequestTimeout int
}

// LoadConfig loads the configuration from the .env file and returns a Config struct.
func LoadConfig(file string) *Config {
	// Load environment variables from .env file and set default values
	err := godotenv.Load(file)
	if err != nil {
		log.Fatal("Error loading .env file", err)
	}
	// use default if not set
	apiVersion, _ := os.LookupEnv("API_VERSION")
	if apiVersion == "" {
		apiVersion = "v1"
	}
	requestTimeout, _ := os.LookupEnv("REQUEST_TIMEOUT")
	if requestTimeout == "" {
		log.Print("REQUEST_TIMEOUT not set, using default value of 5 seconds")
		requestTimeout = "5"
	}
	// convert to int
	intTimeout, err := strconv.Atoi(requestTimeout)
	if err != nil {
		log.Fatal("Error: REQUEST_TIMEOUT should be valid INT", err)
	}

	return &Config{
		APIKey:         os.Getenv("API_KEY"),
		BaseURL:        os.Getenv("API_URL"),
		APIVersion:     apiVersion,
		RequestTimeout: intTimeout,
	}
}
