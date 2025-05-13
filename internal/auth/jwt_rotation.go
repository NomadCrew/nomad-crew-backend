package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/logger"
)

// JWTSecretManager handles JWT secret rotation and management
type JWTSecretManager struct {
	currentSecret   string
	previousSecret  string
	rotationPeriod  time.Duration
	secretMutex     sync.RWMutex
	config          *config.Config
	secretStorageFn func(string) error // Function to store secret in external storage
	lastRotation    time.Time
}

// NewJWTSecretManager creates a new JWT secret manager instance
func NewJWTSecretManager(cfg *config.Config, rotationPeriod time.Duration) *JWTSecretManager {
	// Use existing secret from config as current secret
	return &JWTSecretManager{
		currentSecret:  cfg.Server.JwtSecretKey,
		previousSecret: "",
		rotationPeriod: rotationPeriod,
		config:         cfg,
		lastRotation:   time.Now(),
	}
}

// StartRotation begins periodic JWT secret rotation
func (m *JWTSecretManager) StartRotation(ctx context.Context) {
	log := logger.GetLogger()
	log.Info("Starting JWT secret rotation service")

	ticker := time.NewTicker(m.rotationPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.RotateSecret(); err != nil {
				log.Errorw("Failed to rotate JWT secret", "error", err)
			} else {
				log.Info("JWT secret rotated successfully")
			}
		case <-ctx.Done():
			log.Info("JWT secret rotation service stopped")
			return
		}
	}
}

// RotateSecret generates a new JWT secret and rotates the current one to previous
func (m *JWTSecretManager) RotateSecret() error {
	// Generate new secret
	newSecret, err := generateSecureSecret(32) // 32 bytes = 256 bits
	if err != nil {
		return err
	}

	// Update secrets with lock
	m.secretMutex.Lock()
	m.previousSecret = m.currentSecret
	m.currentSecret = newSecret
	m.lastRotation = time.Now()
	m.secretMutex.Unlock()

	// Store in external storage if configured
	if m.secretStorageFn != nil {
		return m.secretStorageFn(newSecret)
	}

	return nil
}

// GetCurrentSecret returns the current JWT secret
func (m *JWTSecretManager) GetCurrentSecret() string {
	m.secretMutex.RLock()
	defer m.secretMutex.RUnlock()
	return m.currentSecret
}

// GetValidSecrets returns all currently valid secrets (current and previous)
func (m *JWTSecretManager) GetValidSecrets() []string {
	m.secretMutex.RLock()
	defer m.secretMutex.RUnlock()

	// If previous secret exists, return both; otherwise just current
	if m.previousSecret != "" {
		return []string{m.currentSecret, m.previousSecret}
	}
	return []string{m.currentSecret}
}

// SetSecretStorageFunction sets a function to store secrets in external storage
func (m *JWTSecretManager) SetSecretStorageFunction(fn func(string) error) {
	m.secretMutex.Lock()
	defer m.secretMutex.Unlock()
	m.secretStorageFn = fn
}

// Helper function to generate a secure random token
func generateSecureSecret(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
