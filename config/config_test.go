package config_test

import (
	"testing"

	"github.com/netautomate/netorca-go/config"
	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	// Load the configuration from the .env file
	cfg := config.LoadConfig("testdata/.env_example")

	assert.Equal(t, cfg.APIKey, "11.12312312312")
	assert.Equal(t, cfg.BaseURL, "https://api.example.com")
	assert.Equal(t, cfg.APIVersion, "v1")
	assert.Equal(t, cfg.RequestTimeout, 15)
}
