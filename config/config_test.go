package config_test

import (
	"testing"

	"github.com/netautomate/netorca-go/config"
	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	// Load the configuration from the .env file
	cfg := config.LoadConfig("testdata/.env_example")

	assert.Equal(t, "11.12312312312", cfg.APIKey)
	assert.Equal(t, "https://api.example.com", cfg.BaseURL)
	assert.Equal(t, "v1", cfg.APIVersion)
	assert.Equal(t, 15, cfg.RequestTimeout)
}
