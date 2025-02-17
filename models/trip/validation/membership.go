package validation

import (
	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

func ValidateRoleTransition(oldRole, newRole types.MemberRole) error {
	if oldRole == types.MemberRoleOwner && newRole != types.MemberRoleOwner {
		return errors.ValidationFailed(
			"role_transition",
			"Cannot remove owner status from last remaining owner",
		)
	}
	return nil
}

func ValidateMembershipStatus(oldStatus, newStatus types.MembershipStatus) error {
	if oldStatus == types.MembershipStatusInvited && newStatus != types.MembershipStatusActive {
		return  errors.ValidationFailed(
			"status_transition",
			"Invalid status transition for invited member",
		)
	}
	return nil
}
