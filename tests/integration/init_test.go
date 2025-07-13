package integration

import (
	"os"
	"testing"

	"github.com/NomadCrew/nomad-crew-backend/logger"
)

func TestMain(m *testing.M) {
	// Set up test environment
	logger.IsTest = true

	// Run tests
	code := m.Run()

	// Clean up
	os.Exit(code)
}
