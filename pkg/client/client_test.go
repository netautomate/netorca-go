package client_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/netautomate/netorca-go/pkg/client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readTestFile(t *testing.T, filename string) string {
	testFilePath := filepath.Join("testdata", filename)
	content, err := os.ReadFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to read test file %s: %v", testFilePath, err)
	}
	return string(content)
}

func TestClient(t *testing.T) {
	t.Run("Test NewClient returns client on success", func(t *testing.T) {
		client, err := client.NewClient("http://example.com", "test-api-key", "v1", 5*time.Second)
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, "http://example.com/v1/", client.BaseURL)
		assert.Equal(t, "test-api-key", client.APIKey)
	})

	t.Run("Test NewClient returns error on empty base URL", func(t *testing.T) {
		_, err := client.NewClient("", "test-api-key", "v1", 5*time.Second)
		require.Error(t, err)
		assert.Equal(t, "base URL cannot be empty", err.Error())
	})
	t.Run("Test NewClient returns error invalid base URL", func(t *testing.T) {
		_, err := client.NewClient("invalid-url", "test-api-key", "v1", 5*time.Second)
		require.Error(t, err)
		assert.Equal(t, "base URL must start with http:// or https://", err.Error())
	})
}
