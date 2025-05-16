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
	// if oldStatus == types.MembershipStatusInvited && newStatus != types.MembershipStatusActive { // MembershipStatusInvited removed
	// 	return errors.ValidationFailed(
	// 		"status_transition",
	// 		"Invalid status transition for invited member",
	// 	)
	// }
	// TODO: Add any other relevant membership status transition validation if needed.
	// For now, with only ACTIVE/INACTIVE, direct transitions are usually allowed based on auth.
	return nil
}
