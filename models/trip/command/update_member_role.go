package command

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/validation"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
)

type UpdateMemberRoleCommand struct {
	BaseCommand
	TripID   string
	MemberID string
	NewRole  types.MemberRole
}

func (c *UpdateMemberRoleCommand) Validate(ctx context.Context) error {
	if c.TripID == "" {
		return errors.ValidationFailed("trip_id_required", "trip ID is required")
	}
	if c.MemberID == "" {
		return errors.ValidationFailed("member_id_required", "member ID is required")
	}
	if !c.NewRole.IsValid() {
		return errors.ValidationFailed("invalid_member_role", "invalid member role")
	}

	return nil
}

func (c *UpdateMemberRoleCommand) ValidatePermissions(ctx context.Context) error {
	// Requester must be owner to change roles
	return c.BaseCommand.ValidatePermissions(ctx, c.TripID, types.MemberRoleOwner)
}

func (c *UpdateMemberRoleCommand) Execute(ctx context.Context) (*interfaces.CommandResult, error) {
	if err := c.Validate(ctx); err != nil {
		return nil, err
	}
	if err := c.ValidatePermissions(ctx); err != nil {
		return nil, err
	}

	currentRole, err := c.Ctx.Store.GetUserRole(ctx, c.TripID, c.MemberID)
	if err != nil {
		return nil, errors.NotFound("member_not_found", "member not found in trip")
	}

	if err := validation.ValidateRoleTransition(currentRole, c.NewRole); err != nil {
		return nil, err
	}

	// Use the locked method that wraps owner check + role update in a transaction with FOR UPDATE
	// This prevents TOCTOU race conditions on concurrent role changes
	if err := c.Ctx.Store.UpdateMemberRoleWithOwnerLock(ctx, c.TripID, c.MemberID, c.NewRole); err != nil {
		return nil, err
	}

	eventPayload := types.MemberRoleUpdatedEvent{
		MemberID:  c.MemberID,
		OldRole:   currentRole,
		NewRole:   c.NewRole,
		UpdatedBy: c.UserID,
	}

	// Marshal the payload to JSON
	payloadBytes, err := json.Marshal(eventPayload)
	if err != nil {
		return nil, err
	}

	return &interfaces.CommandResult{
		Success: true,
		Data:    eventPayload,
		Events:  []types.Event{createRoleUpdatedEvent(c.TripID, c.UserID, payloadBytes)},
	}, nil
}

func createRoleUpdatedEvent(tripID, userID string, payload json.RawMessage) types.Event {
	return types.Event{
		BaseEvent: types.BaseEvent{
			ID:        uuid.NewString(),
			Type:      types.EventTypeMemberRoleUpdated,
			TripID:    tripID,
			UserID:    userID,
			Timestamp: time.Now(),
			Version:   1,
		},
		Payload: payload,
	}
}
