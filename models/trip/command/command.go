package command

import (
	"context"
	"fmt"
	"sync"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/supabase-community/supabase-go"
)

type Command interface {
	Execute(ctx context.Context) (*interfaces.CommandResult, error)
	Validate(ctx context.Context) error
	ValidatePermissions(ctx context.Context) error
}

type BaseCommand struct {
	CorrelationID string
	UserID        string
	TripID        string
	Ctx           *interfaces.CommandContext
}

// NewCommandContext creates a new thread-safe CommandContext
func NewCommandContext(
	store store.TripStore,
	eventBus types.EventPublisher,
	weatherSvc types.WeatherServiceInterface,
	supabaseClient *supabase.Client,
	config *config.ServerConfig,
	emailsvc types.EmailService,
	chatStore store.ChatStore,
) *interfaces.CommandContext {
	return &interfaces.CommandContext{
		Store:          store,
		EventBus:       eventBus,
		WeatherSvc:     weatherSvc,
		SupabaseClient: supabaseClient,
		Config:         config,
		RequestData:    &sync.Map{},
		EmailSvc:       emailsvc,
		ChatStore:      chatStore,
	}
}

func (c *BaseCommand) ValidatePermissions(ctx context.Context, tripID string, requiredRole types.MemberRole) error {
	// Debug log to check the user ID being used for permission validation
	logger.GetLogger().Debugw("ValidatePermissions called",
		"commandUserID", c.UserID,
		"tripID", tripID,
		"requiredRole", requiredRole,
		"contextUserID", ctx.Value(middleware.UserIDKey))

	role, err := c.Ctx.Store.GetUserRole(ctx, tripID, c.UserID)
	if err != nil {
		logger.GetLogger().Debugw("GetUserRole failed",
			"error", err,
			"commandUserID", c.UserID,
			"tripID", tripID)
		return errors.Forbidden("permission_verification_failed", "Failed to verify permissions")
	}

	logger.GetLogger().Debugw("Role retrieved successfully",
		"role", role,
		"userID", c.UserID,
		"tripID", tripID)

	if !role.IsAuthorizedFor(requiredRole) {
		return errors.Forbidden("insufficient_permissions", fmt.Sprintf("User requires role %s", requiredRole))
	}
	return nil
}
