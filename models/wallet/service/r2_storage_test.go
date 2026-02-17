package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewR2FileStorage_SetsEndpointAndBucket(t *testing.T) {
	storage, err := NewR2FileStorage("test-account-id", "my-bucket", "AKID", "SECRET")
	require.NoError(t, err)
	assert.NotNil(t, storage.client)
	assert.NotNil(t, storage.presigner)
	assert.Equal(t, "my-bucket", storage.bucketName)
}

func TestNewR2FileStorage_EmptyParamsStillConstructs(t *testing.T) {
	// Constructor does not validate credentials — that happens at request time.
	storage, err := NewR2FileStorage("", "", "", "")
	require.NoError(t, err)
	assert.NotNil(t, storage)
}
