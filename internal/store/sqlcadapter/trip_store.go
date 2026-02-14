package sqlcadapter

import (
	"context"
	"errors"
	"fmt"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/sqlc"
	internal_store "github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ensure sqlcTripStore implements internal_store.TripStore
var _ internal_store.TripStore = (*sqlcTripStore)(nil)

type sqlcTripStore struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// NewSqlcTripStore creates a new SQLC-based trip store
func NewSqlcTripStore(pool *pgxpool.Pool) internal_store.TripStore {
	return &sqlcTripStore{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

// GetPool returns the underlying connection pool
func (s *sqlcTripStore) GetPool() *pgxpool.Pool {
	return s.pool
}

// CreateTrip creates a new trip and adds the creator as an owner member
func (s *sqlcTripStore) CreateTrip(ctx context.Context, trip types.Trip) (string, error) {
	log := logger.GetLogger()

	// Start a transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	// Create the trip
	tripID, err := qtx.CreateTrip(ctx, sqlc.CreateTripParams{
		Name:                 trip.Name,
		Description:          StringPtr(trip.Description),
		StartDate:            TimeToPgDate(trip.StartDate),
		EndDate:              TimeToPgDate(trip.EndDate),
		DestinationPlaceID:   trip.DestinationPlaceID,
		DestinationAddress:   trip.DestinationAddress,
		DestinationName:      trip.DestinationName,
		DestinationLatitude:  trip.DestinationLatitude,
		DestinationLongitude: trip.DestinationLongitude,
		CreatedBy:            trip.CreatedBy,
		Status:               TripStatusToSqlc(trip.Status),
		BackgroundImageUrl:   StringPtr(trip.BackgroundImageURL),
	})
	if err != nil {
		return "", fmt.Errorf("failed to insert trip: %w", err)
	}

	// Add creator as owner member
	if trip.CreatedBy != nil {
		err = qtx.CreateMembership(ctx, sqlc.CreateMembershipParams{
			TripID: tripID,
			UserID: *trip.CreatedBy,
			Role:   sqlc.MembershipRoleOWNER,
			Status: sqlc.MembershipStatusACTIVE,
		})
		if err != nil {
			return "", fmt.Errorf("failed to add creator membership: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Infow("Successfully created trip", "tripId", tripID)
	return tripID, nil
}

// GetTrip retrieves a trip by ID
func (s *sqlcTripStore) GetTrip(ctx context.Context, id string) (*types.Trip, error) {
	row, err := s.queries.GetTrip(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("trip", id)
		}
		return nil, fmt.Errorf("failed to get trip: %w", err)
	}

	return GetTripRowToTrip(row), nil
}

// UpdateTrip updates a trip with the provided changes
func (s *sqlcTripStore) UpdateTrip(ctx context.Context, id string, update types.TripUpdate) (*types.Trip, error) {
	log := logger.GetLogger()

	// First, get the current status for validation
	statusRow, err := s.queries.GetTripStatus(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("trip", id)
		}
		return nil, fmt.Errorf("failed to get trip status: %w", err)
	}

	if statusRow.DeletedAt.Valid {
		return nil, apperrors.NotFound("trip", id)
	}

	currentStatus := types.TripStatus(statusRow.Status)

	// Validate status transition if status is being changed
	if update.Status != nil && *update.Status != currentStatus {
		if !currentStatus.IsValidTransition(*update.Status) {
			return nil, apperrors.InvalidStatusTransition(string(currentStatus), string(*update.Status))
		}
	}

	// Apply updates using individual update methods
	if update.Name != nil && *update.Name != "" {
		if err := s.queries.UpdateTripName(ctx, sqlc.UpdateTripNameParams{
			ID:   id,
			Name: *update.Name,
		}); err != nil {
			return nil, fmt.Errorf("failed to update trip name: %w", err)
		}
	}

	if update.Description != nil {
		if err := s.queries.UpdateTripDescription(ctx, sqlc.UpdateTripDescriptionParams{
			ID:          id,
			Description: update.Description,
		}); err != nil {
			return nil, fmt.Errorf("failed to update trip description: %w", err)
		}
	}

	if update.StartDate != nil || update.EndDate != nil {
		// Get current dates if only one is provided
		currentTrip, err := s.queries.GetTrip(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get trip for date update: %w", err)
		}

		startDate := currentTrip.StartDate
		endDate := currentTrip.EndDate

		if update.StartDate != nil {
			startDate = TimeToPgDate(*update.StartDate)
		}
		if update.EndDate != nil {
			endDate = TimeToPgDate(*update.EndDate)
		}

		if err := s.queries.UpdateTripDates(ctx, sqlc.UpdateTripDatesParams{
			ID:        id,
			StartDate: startDate,
			EndDate:   endDate,
		}); err != nil {
			return nil, fmt.Errorf("failed to update trip dates: %w", err)
		}
	}

	if update.Status != nil && *update.Status != "" {
		if err := s.queries.UpdateTripStatus(ctx, sqlc.UpdateTripStatusParams{
			ID:     id,
			Status: TripStatusToSqlc(*update.Status),
		}); err != nil {
			return nil, fmt.Errorf("failed to update trip status: %w", err)
		}
	}

	if update.DestinationPlaceID != nil || update.DestinationAddress != nil ||
		update.DestinationName != nil || update.DestinationLatitude != nil ||
		update.DestinationLongitude != nil {

		currentTrip, err := s.queries.GetTrip(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get trip for destination update: %w", err)
		}

		destPlaceID := currentTrip.DestinationPlaceID
		destAddress := currentTrip.DestinationAddress
		destName := currentTrip.DestinationName
		destLat := currentTrip.DestinationLatitude
		destLng := currentTrip.DestinationLongitude

		if update.DestinationPlaceID != nil {
			destPlaceID = update.DestinationPlaceID
		}
		if update.DestinationAddress != nil {
			destAddress = update.DestinationAddress
		}
		if update.DestinationName != nil {
			destName = update.DestinationName
		}
		if update.DestinationLatitude != nil {
			destLat = *update.DestinationLatitude
		}
		if update.DestinationLongitude != nil {
			destLng = *update.DestinationLongitude
		}

		if err := s.queries.UpdateTripDestination(ctx, sqlc.UpdateTripDestinationParams{
			ID:                   id,
			DestinationPlaceID:   destPlaceID,
			DestinationAddress:   destAddress,
			DestinationName:      destName,
			DestinationLatitude:  destLat,
			DestinationLongitude: destLng,
		}); err != nil {
			return nil, fmt.Errorf("failed to update trip destination: %w", err)
		}
	}

	log.Infow("Trip updated successfully", "tripId", id)
	return s.GetTrip(ctx, id)
}

// SoftDeleteTrip marks a trip as deleted
func (s *sqlcTripStore) SoftDeleteTrip(ctx context.Context, id string) error {
	log := logger.GetLogger()

	if err := s.queries.SoftDeleteTrip(ctx, id); err != nil {
		return fmt.Errorf("failed to soft-delete trip: %w", err)
	}

	log.Infow("Successfully soft-deleted trip", "tripId", id)
	return nil
}

// ListUserTrips retrieves all trips for a user (as member or owner)
func (s *sqlcTripStore) ListUserTrips(ctx context.Context, userID string) ([]*types.Trip, error) {
	rows, err := s.queries.ListUserTrips(ctx, &userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user trips: %w", err)
	}

	trips := make([]*types.Trip, 0, len(rows))
	for _, row := range rows {
		trips = append(trips, ListUserTripsRowToTrip(row))
	}

	return trips, nil
}

// SearchTrips searches trips by criteria
func (s *sqlcTripStore) SearchTrips(ctx context.Context, criteria types.TripSearchCriteria) ([]*types.Trip, error) {
	var destination *string
	if criteria.Destination != "" {
		destination = &criteria.Destination
	}

	// Convert time.Time to pgtype.Date for SQLC
	// Use StartDateFrom if set, otherwise fallback to StartDate
	startDateFromTime := criteria.StartDateFrom
	if startDateFromTime.IsZero() && !criteria.StartDate.IsZero() {
		startDateFromTime = criteria.StartDate
	}

	rows, err := s.queries.SearchTrips(ctx, sqlc.SearchTripsParams{
		Destination:   destination,
		StartDateFrom: OptionalTimeToPgDate(startDateFromTime),
		StartDateTo:   OptionalTimeToPgDate(criteria.StartDateTo),
		EndDateFrom:   InvalidPgDate(), // Not in criteria - pass invalid
		EndDateTo:     OptionalTimeToPgDate(criteria.EndDate),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search trips: %w", err)
	}

	trips := make([]*types.Trip, 0, len(rows))
	for _, row := range rows {
		trips = append(trips, SearchTripsRowToTrip(row))
	}

	return trips, nil
}

// AddMember adds a member to a trip
func (s *sqlcTripStore) AddMember(ctx context.Context, membership *types.TripMembership) error {
	log := logger.GetLogger()

	err := s.queries.UpsertMembership(ctx, sqlc.UpsertMembershipParams{
		TripID: membership.TripID,
		UserID: membership.UserID,
		Role:   MemberRoleToSqlc(membership.Role),
		Status: MemberStatusToSqlc(membership.Status),
	})
	if err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}

	log.Infow("Successfully added member", "tripID", membership.TripID, "userID", membership.UserID)
	return nil
}

// UpdateMemberRole updates a member's role
func (s *sqlcTripStore) UpdateMemberRole(ctx context.Context, tripID string, userID string, role types.MemberRole) error {
	log := logger.GetLogger()

	err := s.queries.UpdateMemberRole(ctx, sqlc.UpdateMemberRoleParams{
		TripID: tripID,
		UserID: userID,
		Role:   MemberRoleToSqlc(role),
	})
	if err != nil {
		return fmt.Errorf("failed to update member role: %w", err)
	}

	log.Infow("Successfully updated member role", "tripID", tripID, "userID", userID, "newRole", role)
	return nil
}

// RemoveMember removes a member from a trip (deactivates membership)
func (s *sqlcTripStore) RemoveMember(ctx context.Context, tripID string, userID string) error {
	log := logger.GetLogger()

	err := s.queries.DeactivateMembership(ctx, sqlc.DeactivateMembershipParams{
		TripID: tripID,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}

	log.Infow("Successfully removed member", "tripID", tripID, "userID", userID)
	return nil
}

// GetTripMembers retrieves all active members of a trip
func (s *sqlcTripStore) GetTripMembers(ctx context.Context, tripID string) ([]types.TripMembership, error) {
	log := logger.GetLogger()

	rows, err := s.queries.GetTripMembers(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("failed to get trip members: %w", err)
	}

	members := make([]types.TripMembership, 0, len(rows))
	for _, row := range rows {
		members = append(members, GetTripMembersRowToMembership(row))
	}

	log.Infow("Successfully retrieved trip members", "tripID", tripID, "count", len(members))
	return members, nil
}

// GetUserRole retrieves the role of a user in a trip
func (s *sqlcTripStore) GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error) {
	role, err := s.queries.GetUserRole(ctx, sqlc.GetUserRoleParams{
		TripID: tripID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperrors.NotFound("membership", fmt.Sprintf("user %s in trip %s", userID, tripID))
		}
		return "", fmt.Errorf("failed to get user role: %w", err)
	}

	return SqlcMemberRoleToTypes(role), nil
}

// LookupUserByEmail looks up a user by email
func (s *sqlcTripStore) LookupUserByEmail(ctx context.Context, email string) (*types.SupabaseUser, error) {
	row, err := s.queries.GetUserProfileByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("User with email", email)
		}
		return nil, fmt.Errorf("failed to lookup user: %w", err)
	}

	return &types.SupabaseUser{
		ID:    row.ID,
		Email: row.Email,
	}, nil
}

// CreateInvitation creates a new trip invitation
func (s *sqlcTripStore) CreateInvitation(ctx context.Context, invitation *types.TripInvitation) error {
	log := logger.GetLogger()

	var inviterID *string
	if invitation.InviterID != "" {
		inviterID = &invitation.InviterID
	}
	// Convert sql.NullString Token to *string for SQLC
	token := NullStringToStringPtr(invitation.Token)

	_, err := s.queries.CreateInvitation(ctx, sqlc.CreateInvitationParams{
		ID:           invitation.ID,
		TripID:       invitation.TripID,
		InviterID:    inviterID,
		InviteeEmail: invitation.InviteeEmail,
		Role:         MemberRoleToSqlc(invitation.Role),
		Token:        token,
		Status:       InvitationStatusToSqlc(invitation.Status),
		ExpiresAt:    TimePtrToPgTimestamptz(invitation.ExpiresAt),
	})
	if err != nil {
		return fmt.Errorf("failed to create invitation: %w", err)
	}

	log.Infow("Successfully created invitation", "invitationID", invitation.ID)
	return nil
}

// GetInvitation retrieves an invitation by ID
func (s *sqlcTripStore) GetInvitation(ctx context.Context, invitationID string) (*types.TripInvitation, error) {
	row, err := s.queries.GetInvitation(ctx, invitationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("Invitation", invitationID)
		}
		return nil, fmt.Errorf("failed to get invitation: %w", err)
	}

	return GetInvitationRowToInvitation(row), nil
}

// GetInvitationsByTripID retrieves all invitations for a trip
func (s *sqlcTripStore) GetInvitationsByTripID(ctx context.Context, tripID string) ([]*types.TripInvitation, error) {
	log := logger.GetLogger()

	rows, err := s.queries.GetInvitationsByTripID(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("failed to get invitations: %w", err)
	}

	invitations := make([]*types.TripInvitation, 0, len(rows))
	for _, row := range rows {
		invitations = append(invitations, GetInvitationsByTripIDRowToInvitation(row))
	}

	log.Infow("Successfully retrieved invitations", "tripID", tripID, "count", len(invitations))
	return invitations, nil
}

// UpdateInvitationStatus updates the status of an invitation
func (s *sqlcTripStore) UpdateInvitationStatus(ctx context.Context, invitationID string, status types.InvitationStatus) error {
	log := logger.GetLogger()

	err := s.queries.UpdateInvitationStatus(ctx, sqlc.UpdateInvitationStatusParams{
		ID:     invitationID,
		Status: InvitationStatusToSqlc(status),
	})
	if err != nil {
		return fmt.Errorf("failed to update invitation status: %w", err)
	}

	log.Infow("Successfully updated invitation status", "invitationID", invitationID, "newStatus", status)
	return nil
}

// AcceptInvitationAtomically adds a member and updates invitation status in a single transaction
func (s *sqlcTripStore) AcceptInvitationAtomically(ctx context.Context, invitationID string, membership *types.TripMembership) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	err = qtx.UpsertMembership(ctx, sqlc.UpsertMembershipParams{
		TripID: membership.TripID,
		UserID: membership.UserID,
		Role:   MemberRoleToSqlc(membership.Role),
		Status: MemberStatusToSqlc(membership.Status),
	})
	if err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}

	err = qtx.UpdateInvitationStatus(ctx, sqlc.UpdateInvitationStatusParams{
		ID:     invitationID,
		Status: InvitationStatusToSqlc(types.InvitationStatusAccepted),
	})
	if err != nil {
		return fmt.Errorf("failed to update invitation status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// RemoveMemberWithOwnerLock removes a member using FOR UPDATE to prevent TOCTOU race on last owner check
func (s *sqlcTripStore) RemoveMemberWithOwnerLock(ctx context.Context, tripID, userID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	// Lock all active members with FOR UPDATE to prevent concurrent modifications
	members, err := qtx.GetTripMembersForUpdate(ctx, tripID)
	if err != nil {
		return fmt.Errorf("failed to get trip members for update: %w", err)
	}

	// Find the target member's role and count owners
	var targetRole sqlc.MembershipRole
	var found bool
	ownerCount := 0
	for _, m := range members {
		if m.Role == sqlc.MembershipRoleOWNER {
			ownerCount++
		}
		if m.UserID == userID {
			targetRole = m.Role
			found = true
		}
	}

	if !found {
		return apperrors.NotFound("membership", fmt.Sprintf("user %s in trip %s", userID, tripID))
	}

	if targetRole == sqlc.MembershipRoleOWNER && ownerCount <= 1 {
		return apperrors.ValidationFailed("Cannot remove last owner", "There must be at least one owner remaining in the trip")
	}

	err = qtx.DeactivateMembership(ctx, sqlc.DeactivateMembershipParams{
		TripID: tripID,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}

	return tx.Commit(ctx)
}

// UpdateMemberRoleWithOwnerLock updates a member role using FOR UPDATE to prevent TOCTOU race on last owner check
func (s *sqlcTripStore) UpdateMemberRoleWithOwnerLock(ctx context.Context, tripID, userID string, newRole types.MemberRole) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	// Lock all active members with FOR UPDATE to prevent concurrent modifications
	members, err := qtx.GetTripMembersForUpdate(ctx, tripID)
	if err != nil {
		return fmt.Errorf("failed to get trip members for update: %w", err)
	}

	// Count owners — only relevant when demoting away from OWNER
	if newRole != types.MemberRoleOwner {
		ownerCount := 0
		for _, m := range members {
			if m.Role == sqlc.MembershipRoleOWNER {
				ownerCount++
			}
		}
		if ownerCount <= 1 {
			// Check if the target user is the last owner
			for _, m := range members {
				if m.UserID == userID && m.Role == sqlc.MembershipRoleOWNER {
					return apperrors.ValidationFailed("last_owner", "Cannot change role of last owner")
				}
			}
		}
	}

	err = qtx.UpdateMemberRole(ctx, sqlc.UpdateMemberRoleParams{
		TripID: tripID,
		UserID: userID,
		Role:   MemberRoleToSqlc(newRole),
	})
	if err != nil {
		return fmt.Errorf("failed to update member role: %w", err)
	}

	return tx.Commit(ctx)
}

// BeginTx starts a new transaction
func (s *sqlcTripStore) BeginTx(ctx context.Context) (types.DatabaseTransaction, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &txWrapper{tx: tx}, nil
}

// Deprecated: Commit is deprecated. Use BeginTx() to start a transaction and call
// Commit() on the returned DatabaseTransaction instead.
func (s *sqlcTripStore) Commit() error {
	logger.GetLogger().Warn("Commit called directly on store, use transaction object instead")
	return fmt.Errorf("commit should be called on transaction object")
}

// Deprecated: Rollback is deprecated. Use BeginTx() to start a transaction and call
// Rollback() on the returned DatabaseTransaction instead.
func (s *sqlcTripStore) Rollback() error {
	logger.GetLogger().Warn("Rollback called directly on store, use transaction object instead")
	return fmt.Errorf("rollback should be called on transaction object")
}

// txWrapper wraps a pgx.Tx to implement types.DatabaseTransaction
type txWrapper struct {
	tx pgx.Tx
}

func (t *txWrapper) Commit() error {
	return t.tx.Commit(context.Background())
}

func (t *txWrapper) Rollback() error {
	return t.tx.Rollback(context.Background())
}
