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

// TodoStatusToSqlc converts types.TodoStatus to sqlc.TodoStatus
func TodoStatusToSqlc(s types.TodoStatus) sqlc.TodoStatus {
	return sqlc.TodoStatus(s)
}

// SqlcTodoStatusToTypes converts sqlc.TodoStatus to types.TodoStatus
func SqlcTodoStatusToTypes(s sqlc.TodoStatus) types.TodoStatus {
	return types.TodoStatus(s)
}

// GetTodoRowToTodo converts sqlc.GetTodoRow to types.Todo
func GetTodoRowToTodo(row *sqlc.GetTodoRow) *types.Todo {
	if row == nil {
		return nil
	}
	return &types.Todo{
		ID:        row.ID,
		TripID:    row.TripID,
		Text:      row.Text,
		Status:    SqlcTodoStatusToTypes(row.Status),
		CreatedBy: row.CreatedBy,
		CreatedAt: PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt: PgTimestamptzToTime(row.UpdatedAt),
	}
}

// ListTodosByTripRowToTodo converts sqlc.ListTodosByTripRow to types.Todo
func ListTodosByTripRowToTodo(row *sqlc.ListTodosByTripRow) *types.Todo {
	if row == nil {
		return nil
	}
	return &types.Todo{
		ID:        row.ID,
		TripID:    row.TripID,
		Text:      row.Text,
		Status:    SqlcTodoStatusToTypes(row.Status),
		CreatedBy: row.CreatedBy,
		CreatedAt: PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt: PgTimestamptzToTime(row.UpdatedAt),
	}
}

// PollStatusToString converts types.PollStatus to string
func PollStatusToString(s types.PollStatus) string {
	return string(s)
}

// StringToPollStatus converts string to types.PollStatus
func StringToPollStatus(s string) types.PollStatus {
	return types.PollStatus(s)
}

// LocationPrivacyToSqlc converts types.LocationPrivacy to sqlc.LocationPrivacy
func LocationPrivacyToSqlc(p types.LocationPrivacy) sqlc.LocationPrivacy {
	return sqlc.LocationPrivacy(p)
}

// SqlcLocationPrivacyToTypes converts sqlc.LocationPrivacy to types.LocationPrivacy
func SqlcLocationPrivacyToTypes(p sqlc.LocationPrivacy) types.LocationPrivacy {
	return types.LocationPrivacy(p)
}

// Float64PtrToFloat64 converts *float64 to float64, returning 0 if nil
func Float64PtrToFloat64(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

// Float64ToFloat64Ptr converts float64 to *float64
func Float64ToFloat64Ptr(f float64) *float64 {
	return &f
}

// BoolPtrToBool converts *bool to bool, returning false if nil
func BoolPtrToBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// BoolToBoolPtr converts bool to *bool
func BoolToBoolPtr(b bool) *bool {
	return &b
}

// GetLocationRowToLocation converts sqlc.GetLocationRow to types.Location
func GetLocationRowToLocation(row *sqlc.GetLocationRow) *types.Location {
	if row == nil {
		return nil
	}
	return &types.Location{
		ID:               row.ID,
		TripID:           row.TripID,
		UserID:           row.UserID,
		Latitude:         row.Latitude,
		Longitude:        row.Longitude,
		Accuracy:         Float64PtrToFloat64(row.Accuracy),
		Timestamp:        PgTimestamptzToTime(row.Timestamp),
		CreatedAt:        PgTimestampToTime(row.CreatedAt),
		UpdatedAt:        PgTimestampToTime(row.UpdatedAt),
		Privacy:          SqlcLocationPrivacyToTypes(row.Privacy),
		IsSharingEnabled: BoolPtrToBool(row.IsSharingEnabled),
		SharingExpiresAt: PgTimestamptzToTimePtr(row.SharingExpiresAt),
	}
}

// GetTripMemberLocationsRowToMemberLocation converts sqlc.GetTripMemberLocationsRow to types.MemberLocation
func GetTripMemberLocationsRowToMemberLocation(row *sqlc.GetTripMemberLocationsRow) *types.MemberLocation {
	if row == nil {
		return nil
	}
	return &types.MemberLocation{
		Location: types.Location{
			ID:               row.ID,
			TripID:           row.TripID,
			UserID:           row.UserID,
			Latitude:         row.Latitude,
			Longitude:        row.Longitude,
			Accuracy:         Float64PtrToFloat64(row.Accuracy),
			Timestamp:        PgTimestamptzToTime(row.Timestamp),
			CreatedAt:        PgTimestampToTime(row.CreatedAt),
			UpdatedAt:        PgTimestampToTime(row.UpdatedAt),
			Privacy:          SqlcLocationPrivacyToTypes(row.Privacy),
			IsSharingEnabled: BoolPtrToBool(row.IsSharingEnabled),
			SharingExpiresAt: PgTimestamptzToTimePtr(row.SharingExpiresAt),
		},
		// UserName and UserRole will need to be populated from user data
		UserName: "",
		UserRole: "",
	}
}

// NotificationTypeToSqlc converts types.NotificationType to sqlc.NotificationType
func NotificationTypeToSqlc(t types.NotificationType) sqlc.NotificationType {
	return sqlc.NotificationType(t)
}

// SqlcNotificationTypeToTypes converts sqlc.NotificationType to types.NotificationType
func SqlcNotificationTypeToTypes(t sqlc.NotificationType) types.NotificationType {
	return types.NotificationType(t)
}

// SqlcNotificationToTypes converts sqlc.Notification to types.Notification
func SqlcNotificationToTypes(n *sqlc.Notification) *types.Notification {
	if n == nil {
		return nil
	}
	return &types.Notification{
		ID:        n.ID,
		UserID:    n.UserID,
		Type:      SqlcNotificationTypeToTypes(n.Type),
		Metadata:  n.Metadata,
		IsRead:    n.IsRead,
		CreatedAt: PgTimestamptzToTime(n.CreatedAt),
		UpdatedAt: PgTimestamptzToTime(n.UpdatedAt),
	}
}

// User converters

// GetUserProfileByIDRowToUser converts sqlc.GetUserProfileByIDRow to types.User
func GetUserProfileByIDRowToUser(row *sqlc.GetUserProfileByIDRow) *types.User {
	if row == nil {
		return nil
	}
	return &types.User{
		ID:                row.ID,
		Email:             row.Email,
		Username:          row.Username,
		FirstName:         row.FirstName,
		LastName:          row.LastName,
		ProfilePictureURL: row.AvatarUrl,
		ContactEmail:      row.ContactEmail,
		CreatedAt:         PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:         PgTimestamptzToTime(row.UpdatedAt),
	}
}

// GetUserProfileByEmailRowToUser converts sqlc.GetUserProfileByEmailRow to types.User
func GetUserProfileByEmailRowToUser(row *sqlc.GetUserProfileByEmailRow) *types.User {
	if row == nil {
		return nil
	}
	return &types.User{
		ID:                row.ID,
		Email:             row.Email,
		Username:          row.Username,
		FirstName:         row.FirstName,
		LastName:          row.LastName,
		ProfilePictureURL: row.AvatarUrl,
		ContactEmail:      row.ContactEmail,
		CreatedAt:         PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:         PgTimestamptzToTime(row.UpdatedAt),
	}
}

// GetUserProfileByUsernameRowToUser converts sqlc.GetUserProfileByUsernameRow to types.User
func GetUserProfileByUsernameRowToUser(row *sqlc.GetUserProfileByUsernameRow) *types.User {
	if row == nil {
		return nil
	}
	return &types.User{
		ID:                row.ID,
		Email:             row.Email,
		Username:          row.Username,
		FirstName:         row.FirstName,
		LastName:          row.LastName,
		ProfilePictureURL: row.AvatarUrl,
		ContactEmail:      row.ContactEmail,
		CreatedAt:         PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:         PgTimestamptzToTime(row.UpdatedAt),
	}
}

// ListUserProfilesRowToUser converts sqlc.ListUserProfilesRow to types.User
func ListUserProfilesRowToUser(row *sqlc.ListUserProfilesRow) *types.User {
	if row == nil {
		return nil
	}
	return &types.User{
		ID:                row.ID,
		Email:             row.Email,
		Username:          row.Username,
		FirstName:         row.FirstName,
		LastName:          row.LastName,
		ProfilePictureURL: row.AvatarUrl,
		ContactEmail:      row.ContactEmail,
		CreatedAt:         PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:         PgTimestamptzToTime(row.UpdatedAt),
	}
}

// SearchUserProfilesRowToUserSearchResult converts sqlc.SearchUserProfilesRow to types.UserSearchResult
func SearchUserProfilesRowToUserSearchResult(row *sqlc.SearchUserProfilesRow) *types.UserSearchResult {
	if row == nil {
		return nil
	}
	return &types.UserSearchResult{
		ID:           row.ID,
		Email:        row.Email,
		Username:     row.Username,
		FirstName:    row.FirstName,
		LastName:     row.LastName,
		AvatarURL:    row.AvatarUrl,
		ContactEmail: DerefString(row.ContactEmail),
	}
}

// GetUserProfileByContactEmailRowToUser converts sqlc.GetUserProfileByContactEmailRow to types.User
func GetUserProfileByContactEmailRowToUser(row *sqlc.GetUserProfileByContactEmailRow) *types.User {
	if row == nil {
		return nil
	}
	return &types.User{
		ID:                row.ID,
		Email:             row.Email,
		Username:          row.Username,
		FirstName:         row.FirstName,
		LastName:          row.LastName,
		ProfilePictureURL: row.AvatarUrl,
		ContactEmail:      row.ContactEmail,
		CreatedAt:         PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:         PgTimestamptzToTime(row.UpdatedAt),
	}
}
