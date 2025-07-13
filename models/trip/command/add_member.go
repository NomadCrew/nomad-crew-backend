package command

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
)

type AddMemberCommand struct {
	BaseCommand
	TripID string
	UserID string
	Role   types.MemberRole
}

func (c *AddMemberCommand) Validate(ctx context.Context) error {
	if c.TripID == "" {
		return errors.ValidationFailed("trip_id_required", "Trip ID is required")
	}
	if c.UserID == "" {
		return errors.ValidationFailed("user_id_required", "user ID is required")
	}
	if !c.Role.IsValid() {
		return errors.ValidationFailed("invalid_role", "Invalid member role")
	}
	return nil
}

func (c *AddMemberCommand) ValidatePermissions(ctx context.Context) error {
	return c.BaseCommand.ValidatePermissions(ctx, c.TripID, types.MemberRoleOwner)
}

func (c *AddMemberCommand) Execute(ctx context.Context) (*interfaces.CommandResult, error) {
	if err := c.Validate(ctx); err != nil {
		return nil, err
	}
	if err := c.ValidatePermissions(ctx); err != nil {
		return nil, err
	}

	// Check existing membership
	if _, err := c.Ctx.Store.GetUserRole(ctx, c.TripID, c.UserID); err == nil {
		return nil, errors.NewConflictError("user_already_member", "User is already a member of this trip")
	}

	membership := &types.TripMembership{
		TripID: c.TripID,
		UserID: c.UserID,
		Role:   c.Role,
		Status: types.MembershipStatusActive,
	}

	if err := c.Ctx.Store.AddMember(ctx, membership); err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(membership)

	return &interfaces.CommandResult{
		Success: true,
		Data:    membership,
		Events: []types.Event{{
			BaseEvent: types.BaseEvent{
				ID:        uuid.NewString(),
				Type:      types.EventTypeMemberAdded,
				TripID:    c.TripID,
				UserID:    c.UserID,
				Timestamp: time.Now(),
				Version:   1,
			},
			Payload: payload,
		}},
	}, nil
}
