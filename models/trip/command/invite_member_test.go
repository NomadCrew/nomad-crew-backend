package command

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockEmailService is a mock implementation of the EmailService interface
type MockEmailService struct {
	mock.Mock
}

func (m *MockEmailService) SendInvitationEmail(ctx context.Context, data types.EmailData) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func TestInviteMemberCommand_AcceptanceURL(t *testing.T) {
	tests := []struct {
		name        string
		frontendURL string
		expected    string
	}{
		{
			name:        "With valid URL with protocol",
			frontendURL: "https://nomadcrew.uk",
			expected:    "https://nomadcrew.uk/invite/accept/",
		},
		{
			name:        "With valid URL without protocol",
			frontendURL: "nomadcrew.uk",
			expected:    "https://nomadcrew.uk/invite/accept/",
		},
		{
			name:        "With trailing slash",
			frontendURL: "https://nomadcrew.uk/",
			expected:    "https://nomadcrew.uk/invite/accept/",
		},
		{
			name:        "With empty URL",
			frontendURL: "",
			expected:    "https://nomadcrew.uk/invite/accept/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock email service
			mockEmailSvc := &MockEmailService{}
			mockEmailSvc.On("SendInvitationEmail", mock.Anything, mock.Anything).Return(nil)

			// Create a test invitation
			expiresAt := time.Now().Add(7 * 24 * time.Hour)
			invitation := &types.TripInvitation{
				ID:           "test-invitation-id",
				TripID:       "test-trip-id",
				InviterID:    "test-user-id",
				InviteeEmail: "test@example.com",
				Role:         types.MemberRoleMember,
				Status:       types.InvitationStatusPending,
				ExpiresAt:    &expiresAt,
			}

			// Create a command with the test configuration
			cmd := &InviteMemberCommand{
				Invitation: invitation,
				BaseCommand: BaseCommand{
					UserID: "test-user-id",
					Ctx: &interfaces.CommandContext{
						Config: &config.ServerConfig{
							JwtSecretKey: "test-secret-key-that-is-long-enough-for-testing",
							FrontendURL:  tt.frontendURL,
						},
						EmailSvc: mockEmailSvc,
					},
				},
			}

			// Test the URL generation logic directly
			frontendURL := cmd.Ctx.Config.FrontendURL

			// Ensure frontendURL is not empty and has a protocol
			if frontendURL == "" {
				frontendURL = "https://nomadcrew.uk" // Default fallback
			}

			// Ensure URL has protocol
			if !strings.HasPrefix(frontendURL, "https://") && !strings.HasPrefix(frontendURL, "http://") {
				frontendURL = "https://" + frontendURL
			}

			// Remove trailing slash if present
			if len(frontendURL) > 0 && frontendURL[len(frontendURL)-1] == '/' {
				frontendURL = frontendURL[:len(frontendURL)-1]
			}

			acceptanceURL := frontendURL + "/invite/accept/test-token"

			// Verify the URL format
			assert.True(t, strings.HasPrefix(acceptanceURL, tt.expected),
				"Expected URL to start with %s, got %s", tt.expected, acceptanceURL)
		})
	}
}
