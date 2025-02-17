package command

import (
	"context"
	"fmt"
	"sync"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/shared"
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
	Ctx           *interfaces.CommandContext
}

// NewCommandContext creates a new thread-safe CommandContext
func NewCommandContext(
	store store.TripStore,
	eventBus types.EventPublisher,
	weatherSvc types.WeatherServiceInterface,
	supabaseClient *supabase.Client,
) *interfaces.CommandContext {
	return &interfaces.CommandContext{
		Store:          store,
		EventBus:       eventBus,
		WeatherSvc:     weatherSvc,
		SupabaseClient: supabaseClient,
		RequestData:    &sync.Map{},
	}
}

func (c *BaseCommand) emitEvent(ctx context.Context, tripID string, eventType types.EventType, payload interface{}) error {
	emitter := shared.NewEventEmitter(c.Ctx.EventBus)
	return emitter.EmitTripEvent(
		ctx,
		tripID,
		eventType,
		payload,
		c.UserID,
	)
}

func (c *BaseCommand) ValidatePermissions(ctx context.Context, tripID string, requiredRole types.MemberRole) error {
	role, err := c.Ctx.Store.GetUserRole(ctx, tripID, c.UserID)
	if err != nil {
		return errors.Forbidden("permission_verification_failed", "Failed to verify permissions")
	}

	if !role.IsAuthorizedFor(requiredRole) {
		return errors.Forbidden("insufficient_permissions", fmt.Sprintf("User requires role %s", requiredRole))
	}
	return nil
}
