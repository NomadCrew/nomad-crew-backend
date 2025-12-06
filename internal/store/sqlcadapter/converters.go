// Package sqlcadapter provides adapters to convert between SQLC generated types
// and the application's domain types.
package sqlcadapter

import (
	"database/sql"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/internal/sqlc"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v5/pgtype"
)

// TimeToPgDate converts time.Time to pgtype.Date
func TimeToPgDate(t time.Time) pgtype.Date {
	return pgtype.Date{
		Time:  t,
		Valid: true,
	}
}

// OptionalTimeToPgDate converts time.Time to pgtype.Date, returning invalid if zero
func OptionalTimeToPgDate(t time.Time) pgtype.Date {
	if t.IsZero() {
		return pgtype.Date{Valid: false}
	}
	return pgtype.Date{
		Time:  t,
		Valid: true,
	}
}

// InvalidPgDate returns an invalid pgtype.Date
func InvalidPgDate() pgtype.Date {
	return pgtype.Date{Valid: false}
}

// PgDateToTime converts pgtype.Date to time.Time
func PgDateToTime(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return d.Time
}

// TimeToPgTimestamp converts time.Time to pgtype.Timestamp
func TimeToPgTimestamp(t time.Time) pgtype.Timestamp {
	return pgtype.Timestamp{
		Time:  t,
		Valid: true,
	}
}

// PgTimestampToTime converts pgtype.Timestamp to time.Time
func PgTimestampToTime(ts pgtype.Timestamp) time.Time {
	if !ts.Valid {
		return time.Time{}
	}
	return ts.Time
}

// TimeToPgTimestamptz converts time.Time to pgtype.Timestamptz
func TimeToPgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  t,
		Valid: true,
	}
}

// PgTimestamptzToTimePtr converts pgtype.Timestamptz to *time.Time
func PgTimestamptzToTimePtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	return &ts.Time
}

// PgTimestamptzToTime converts pgtype.Timestamptz to time.Time (returns zero time if invalid)
func PgTimestamptzToTime(ts pgtype.Timestamptz) time.Time {
	if !ts.Valid {
		return time.Time{}
	}
	return ts.Time
}

// StringPtrToNullString converts *string to sql.NullString
func StringPtrToNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

// NullStringToStringPtr converts sql.NullString to *string
func NullStringToStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

// TimePtrToPgTimestamptz converts *time.Time to pgtype.Timestamptz
func TimePtrToPgTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{
		Time:  *t,
		Valid: true,
	}
}

// StringPtr returns a pointer to the string, or nil if empty
func StringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// DerefString dereferences a string pointer, returning empty string if nil
func DerefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// TripStatusToSqlc converts types.TripStatus to sqlc.TripStatus
func TripStatusToSqlc(s types.TripStatus) sqlc.TripStatus {
	return sqlc.TripStatus(s)
}

// SqlcTripStatusToTypes converts sqlc.TripStatus to types.TripStatus
func SqlcTripStatusToTypes(s sqlc.TripStatus) types.TripStatus {
	return types.TripStatus(s)
}

// MemberRoleToSqlc converts types.MemberRole to sqlc.MembershipRole
func MemberRoleToSqlc(r types.MemberRole) sqlc.MembershipRole {
	return sqlc.MembershipRole(r)
}

// SqlcMemberRoleToTypes converts sqlc.MembershipRole to types.MemberRole
func SqlcMemberRoleToTypes(r sqlc.MembershipRole) types.MemberRole {
	return types.MemberRole(r)
}

// MemberStatusToSqlc converts types.MembershipStatus to sqlc.MembershipStatus
func MemberStatusToSqlc(s types.MembershipStatus) sqlc.MembershipStatus {
	return sqlc.MembershipStatus(s)
}

// SqlcMemberStatusToTypes converts sqlc.MembershipStatus to types.MembershipStatus
func SqlcMemberStatusToTypes(s sqlc.MembershipStatus) types.MembershipStatus {
	return types.MembershipStatus(s)
}

// InvitationStatusToSqlc converts types.InvitationStatus to sqlc.InvitationStatus
func InvitationStatusToSqlc(s types.InvitationStatus) sqlc.InvitationStatus {
	return sqlc.InvitationStatus(s)
}

// SqlcInvitationStatusToTypes converts sqlc.InvitationStatus to types.InvitationStatus
func SqlcInvitationStatusToTypes(s sqlc.InvitationStatus) types.InvitationStatus {
	return types.InvitationStatus(s)
}

// GetTripRowToTrip converts sqlc.GetTripRow to types.Trip
func GetTripRowToTrip(row *sqlc.GetTripRow) *types.Trip {
	if row == nil {
		return nil
	}
	return &types.Trip{
		ID:                   row.ID,
		Name:                 row.Name,
		Description:          DerefString(row.Description),
		DestinationPlaceID:   row.DestinationPlaceID,
		DestinationAddress:   row.DestinationAddress,
		DestinationName:      row.DestinationName,
		DestinationLatitude:  row.DestinationLatitude,
		DestinationLongitude: row.DestinationLongitude,
		StartDate:            PgDateToTime(row.StartDate),
		EndDate:              PgDateToTime(row.EndDate),
		Status:               SqlcTripStatusToTypes(row.Status),
		CreatedBy:            row.CreatedBy,
		CreatedAt:            PgTimestampToTime(row.CreatedAt),
		UpdatedAt:            PgTimestampToTime(row.UpdatedAt),
		DeletedAt:            PgTimestamptzToTimePtr(row.DeletedAt),
		BackgroundImageURL:   row.BackgroundImageUrl,
	}
}

// ListUserTripsRowToTrip converts sqlc.ListUserTripsRow to types.Trip
func ListUserTripsRowToTrip(row *sqlc.ListUserTripsRow) *types.Trip {
	if row == nil {
		return nil
	}
	return &types.Trip{
		ID:                   row.ID,
		Name:                 row.Name,
		Description:          DerefString(row.Description),
		DestinationPlaceID:   row.DestinationPlaceID,
		DestinationAddress:   row.DestinationAddress,
		DestinationName:      row.DestinationName,
		DestinationLatitude:  row.DestinationLatitude,
		DestinationLongitude: row.DestinationLongitude,
		StartDate:            PgDateToTime(row.StartDate),
		EndDate:              PgDateToTime(row.EndDate),
		Status:               SqlcTripStatusToTypes(row.Status),
		CreatedBy:            row.CreatedBy,
		CreatedAt:            PgTimestampToTime(row.CreatedAt),
		UpdatedAt:            PgTimestampToTime(row.UpdatedAt),
		DeletedAt:            PgTimestamptzToTimePtr(row.DeletedAt),
		BackgroundImageURL:   row.BackgroundImageUrl,
	}
}

// ListMemberTripsRowToTrip converts sqlc.ListMemberTripsRow to types.Trip
func ListMemberTripsRowToTrip(row *sqlc.ListMemberTripsRow) *types.Trip {
	if row == nil {
		return nil
	}
	return &types.Trip{
		ID:                   row.ID,
		Name:                 row.Name,
		Description:          DerefString(row.Description),
		DestinationPlaceID:   row.DestinationPlaceID,
		DestinationAddress:   row.DestinationAddress,
		DestinationName:      row.DestinationName,
		DestinationLatitude:  row.DestinationLatitude,
		DestinationLongitude: row.DestinationLongitude,
		StartDate:            PgDateToTime(row.StartDate),
		EndDate:              PgDateToTime(row.EndDate),
		Status:               SqlcTripStatusToTypes(row.Status),
		CreatedBy:            row.CreatedBy,
		CreatedAt:            PgTimestampToTime(row.CreatedAt),
		UpdatedAt:            PgTimestampToTime(row.UpdatedAt),
		DeletedAt:            PgTimestamptzToTimePtr(row.DeletedAt),
		BackgroundImageURL:   row.BackgroundImageUrl,
	}
}

// SearchTripsRowToTrip converts sqlc.SearchTripsRow to types.Trip
func SearchTripsRowToTrip(row *sqlc.SearchTripsRow) *types.Trip {
	if row == nil {
		return nil
	}
	return &types.Trip{
		ID:                   row.ID,
		Name:                 row.Name,
		Description:          DerefString(row.Description),
		DestinationPlaceID:   row.DestinationPlaceID,
		DestinationAddress:   row.DestinationAddress,
		DestinationName:      row.DestinationName,
		DestinationLatitude:  row.DestinationLatitude,
		DestinationLongitude: row.DestinationLongitude,
		StartDate:            PgDateToTime(row.StartDate),
		EndDate:              PgDateToTime(row.EndDate),
		Status:               SqlcTripStatusToTypes(row.Status),
		CreatedBy:            row.CreatedBy,
		CreatedAt:            PgTimestampToTime(row.CreatedAt),
		UpdatedAt:            PgTimestampToTime(row.UpdatedAt),
		DeletedAt:            PgTimestamptzToTimePtr(row.DeletedAt),
		BackgroundImageURL:   row.BackgroundImageUrl,
	}
}

// GetTripMembersRowToMembership converts sqlc.GetTripMembersRow to types.TripMembership
func GetTripMembersRowToMembership(row *sqlc.GetTripMembersRow) types.TripMembership {
	return types.TripMembership{
		ID:        row.ID,
		TripID:    row.TripID,
		UserID:    row.UserID,
		Role:      SqlcMemberRoleToTypes(row.Role),
		Status:    SqlcMemberStatusToTypes(row.Status),
		CreatedAt: PgTimestampToTime(row.CreatedAt),
		UpdatedAt: PgTimestampToTime(row.UpdatedAt),
	}
}

// GetInvitationRowToInvitation converts sqlc.GetInvitationRow to types.TripInvitation
func GetInvitationRowToInvitation(row *sqlc.GetInvitationRow) *types.TripInvitation {
	if row == nil {
		return nil
	}
	return &types.TripInvitation{
		ID:           row.ID,
		TripID:       row.TripID,
		InviterID:    DerefString(row.InviterID),
		InviteeEmail: row.InviteeEmail,
		Role:         SqlcMemberRoleToTypes(row.Role),
		Token:        StringPtrToNullString(row.Token),
		Status:       SqlcInvitationStatusToTypes(row.Status),
		ExpiresAt:    PgTimestamptzToTimePtr(row.ExpiresAt),
		CreatedAt:    PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:    PgTimestamptzToTime(row.UpdatedAt),
	}
}

// GetInvitationsByTripIDRowToInvitation converts sqlc.GetInvitationsByTripIDRow to types.TripInvitation
func GetInvitationsByTripIDRowToInvitation(row *sqlc.GetInvitationsByTripIDRow) *types.TripInvitation {
	if row == nil {
		return nil
	}
	return &types.TripInvitation{
		ID:           row.ID,
		TripID:       row.TripID,
		InviterID:    DerefString(row.InviterID),
		InviteeEmail: row.InviteeEmail,
		Role:         SqlcMemberRoleToTypes(row.Role),
		Token:        StringPtrToNullString(row.Token),
		Status:       SqlcInvitationStatusToTypes(row.Status),
		ExpiresAt:    PgTimestamptzToTimePtr(row.ExpiresAt),
		CreatedAt:    PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:    PgTimestamptzToTime(row.UpdatedAt),
	}
}
