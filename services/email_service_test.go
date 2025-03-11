package services

import (
	"context"
	"testing"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/resend/resend-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock Resend client
type mockEmailsService struct {
	mock.Mock
}

func (m *mockEmailsService) Send(params *resend.SendEmailRequest) (*resend.SendEmailResponse, error) {
	args := m.Called(params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resend.SendEmailResponse), args.Error(1)
}

func (m *mockEmailsService) SendWithContext(ctx context.Context, params *resend.SendEmailRequest) (*resend.SendEmailResponse, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resend.SendEmailResponse), args.Error(1)
}

func (m *mockEmailsService) Update(params *resend.UpdateEmailRequest) (*resend.UpdateEmailResponse, error) {
	args := m.Called(params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resend.UpdateEmailResponse), args.Error(1)
}

func (m *mockEmailsService) UpdateWithContext(ctx context.Context, params *resend.UpdateEmailRequest) (*resend.UpdateEmailResponse, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resend.UpdateEmailResponse), args.Error(1)
}

func (m *mockEmailsService) Cancel(id string) (*resend.CancelScheduledEmailResponse, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resend.CancelScheduledEmailResponse), args.Error(1)
}

func (m *mockEmailsService) CancelWithContext(ctx context.Context, id string) (*resend.CancelScheduledEmailResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resend.CancelScheduledEmailResponse), args.Error(1)
}

func (m *mockEmailsService) Get(id string) (*resend.Email, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resend.Email), args.Error(1)
}

func (m *mockEmailsService) GetWithContext(ctx context.Context, id string) (*resend.Email, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resend.Email), args.Error(1)
}

// Mock registry that doesn't actually register metrics
type mockRegistry struct{}

func (m *mockRegistry) Register(c prometheus.Collector) error   { return nil }
func (m *mockRegistry) MustRegister(cs ...prometheus.Collector) {}
func (m *mockRegistry) Unregister(c prometheus.Collector) bool  { return true }

func TestNewEmailService(t *testing.T) {
	cfg := &config.EmailConfig{
		FromName:     "Test Sender",
		FromAddress:  "test@example.com",
		ResendAPIKey: "test-api-key",
	}

	service := NewEmailService(cfg)

	assert.NotNil(t, service)
	assert.Equal(t, cfg, service.config)
	assert.NotNil(t, service.client)
	assert.NotNil(t, service.metrics)
}

func TestSendInvitationEmail(t *testing.T) {
	tests := []struct {
		name        string
		emailData   types.EmailData
		setupMock   func(*mockEmailsService)
		expectError bool
	}{
		{
			name: "successful email send",
			emailData: types.EmailData{
				To:      "recipient@example.com",
				Subject: "Test Invitation",
				TemplateData: map[string]interface{}{
					"UserEmail":     "user@example.com",
					"TripName":      "Test Trip",
					"AcceptanceURL": "http://example.com/accept",
				},
			},
			setupMock: func(m *mockEmailsService) {
				m.On("Send", mock.AnythingOfType("*resend.SendEmailRequest")).
					Return(&resend.SendEmailResponse{Id: "test-id"}, nil)
			},
			expectError: false,
		},
		{
			name: "failed email send",
			emailData: types.EmailData{
				To:      "recipient@example.com",
				Subject: "Test Invitation",
				TemplateData: map[string]interface{}{
					"UserEmail":     "user@example.com",
					"TripName":      "Test Trip",
					"AcceptanceURL": "http://example.com/accept",
				},
			},
			setupMock: func(m *mockEmailsService) {
				m.On("Send", mock.AnythingOfType("*resend.SendEmailRequest")).
					Return(nil, assert.AnError)
			},
			expectError: true,
		},
		{
			name: "invalid template data",
			emailData: types.EmailData{
				To:      "recipient@example.com",
				Subject: "Test Invitation",
				TemplateData: map[string]interface{}{
					"InvalidKey": "This will cause template execution to fail",
					// Missing required fields: UserEmail, TripName, AcceptanceURL
				},
			},
			setupMock: func(m *mockEmailsService) {
				// Mock should not be called since validation will fail first
			},
			expectError: true,
		},
		{
			name: "missing required template field",
			emailData: types.EmailData{
				To:      "recipient@example.com",
				Subject: "Test Invitation",
				TemplateData: map[string]interface{}{
					"UserEmail": "user@example.com",
					"TripName":  "Test Trip",
					// Missing AcceptanceURL
				},
			},
			setupMock: func(m *mockEmailsService) {
				// Mock should not be called since validation will fail first
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockEmails := &mockEmailsService{}
			if tt.setupMock != nil {
				tt.setupMock(mockEmails)
			}

			cfg := &config.EmailConfig{
				FromName:     "Test Sender",
				FromAddress:  "test@example.com",
				ResendAPIKey: "test-api-key",
			}

			service := NewEmailServiceWithRegistry(cfg, &mockRegistry{})
			service.client.Emails = mockEmails

			// Execute
			err := service.SendInvitationEmail(context.Background(), tt.emailData)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockEmails.AssertExpectations(t)
		})
	}
}

func TestEmailMetrics(t *testing.T) {
	cfg := &config.EmailConfig{
		FromName:     "Test Sender",
		FromAddress:  "test@example.com",
		ResendAPIKey: "test-api-key",
	}

	service := NewEmailServiceWithRegistry(cfg, &mockRegistry{})
	mockEmails := &mockEmailsService{}
	service.client.Emails = mockEmails

	// Test successful send metrics
	mockEmails.On("Send", mock.AnythingOfType("*resend.SendEmailRequest")).
		Return(&resend.SendEmailResponse{Id: "test-id"}, nil).Once()

	emailData := types.EmailData{
		To:      "recipient@example.com",
		Subject: "Test Invitation",
		TemplateData: map[string]interface{}{
			"UserEmail":     "user@example.com",
			"TripName":      "Test Trip",
			"AcceptanceURL": "http://example.com/accept",
		},
	}

	// Initial metric values
	initialSentCount := testGetCounterValue(service.metrics.sentCount)
	initialErrorCount := testGetCounterValue(service.metrics.errorCount)

	err := service.SendInvitationEmail(context.Background(), emailData)
	assert.NoError(t, err)

	// Check metrics after successful send
	assert.Equal(t, initialSentCount+1, testGetCounterValue(service.metrics.sentCount))
	assert.Equal(t, initialErrorCount, testGetCounterValue(service.metrics.errorCount))

	// Test error metrics with missing template data
	invalidEmailData := types.EmailData{
		To:           "recipient@example.com",
		Subject:      "Test Invitation",
		TemplateData: map[string]interface{}{
			// Missing required fields
		},
	}

	err = service.SendInvitationEmail(context.Background(), invalidEmailData)
	assert.Error(t, err)

	// Check metrics after template validation error
	assert.Equal(t, initialSentCount+1, testGetCounterValue(service.metrics.sentCount))
	assert.Equal(t, initialErrorCount+1, testGetCounterValue(service.metrics.errorCount))

	// Test error metrics with send failure
	mockEmails.On("Send", mock.AnythingOfType("*resend.SendEmailRequest")).
		Return(nil, assert.AnError).Once()

	err = service.SendInvitationEmail(context.Background(), emailData)
	assert.Error(t, err)

	// Check metrics after send error
	assert.Equal(t, initialSentCount+1, testGetCounterValue(service.metrics.sentCount))
	assert.Equal(t, initialErrorCount+2, testGetCounterValue(service.metrics.errorCount))

	mockEmails.AssertExpectations(t)
}

// Helper function to get counter value
func testGetCounterValue(counter prometheus.Counter) float64 {
	var m dto.Metric
	counter.Write(&m)
	return *m.Counter.Value
}
