package validation

import (
	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// ValidateRoleTransition enforces OWNER immutability:
// - OWNER role cannot be changed via API
// - OWNER role cannot be assigned via API
// The OWNER is set at trip creation time and is immutable.
func ValidateRoleTransition(oldRole, newRole types.MemberRole) error {
	// Cannot change OWNER's role - OWNER is immutable
	if oldRole == types.MemberRoleOwner {
		return errors.Forbidden(
			"owner_immutable",
			"Owner role cannot be changed. The trip owner is set at creation and is immutable.",
		)
	}

	// Cannot assign OWNER role to anyone - OWNER can only be set at trip creation
	if newRole == types.MemberRoleOwner {
		return errors.Forbidden(
			"owner_immutable",
			"Owner role cannot be assigned. The trip owner is set at creation and is immutable.",
		)
	}

	return nil
}

// ValidateMembershipStatus validates membership status transitions.
// With only ACTIVE and INACTIVE statuses, transitions are controlled by authorization
// rather than state machine validation. No additional validation is needed.
func ValidateMembershipStatus(oldStatus, newStatus types.MembershipStatus) error {
	return nil
}
