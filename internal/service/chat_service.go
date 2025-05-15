package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/internal/utils"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"go.uber.org/zap"
)

// ChatService defines the interface for chat-related business logic.
type ChatService interface {
	// Group Operations
	CreateGroup(ctx context.Context, tripID, groupName, createdByUserID string) (*types.ChatGroup, error)
	GetGroup(ctx context.Context, groupID, requestingUserID string) (*types.ChatGroup, error)
	UpdateGroup(ctx context.Context, groupID, requestingUserID string, updateReq types.ChatGroupUpdateRequest) (*types.ChatGroup, error)
	DeleteGroup(ctx context.Context, groupID, requestingUserID string) error
	ListTripGroups(ctx context.Context, tripID, requestingUserID string, params types.PaginationParams) (*types.ChatGroupPaginatedResponse, error)

	// Member Operations
	AddMember(ctx context.Context, groupID, actorUserID, targetUserID string) error
	RemoveMember(ctx context.Context, groupID, actorUserID, targetUserID string) error
	ListMembers(ctx context.Context, groupID, requestingUserID string) ([]types.UserResponse, error)

	// Message Operations
	PostMessage(ctx context.Context, groupID, userID, content string) (*types.ChatMessageWithUser, error)
	GetMessage(ctx context.Context, messageID, requestingUserID string) (*types.ChatMessageWithUser, error)
	UpdateMessage(ctx context.Context, messageID, requestingUserID, newContent string) (*types.ChatMessageWithUser, error)
	DeleteMessage(ctx context.Context, messageID, requestingUserID string) error
	ListMessages(ctx context.Context, groupID, requestingUserID string, params types.PaginationParams) (*types.ChatMessagePaginatedResponse, error)

	// Reaction Operations
	AddReaction(ctx context.Context, messageID, userID, reaction string) error
	RemoveReaction(ctx context.Context, messageID, userID, reaction string) error

	// Read Status Operations
	UpdateLastRead(ctx context.Context, groupID, userID, messageID string) error

	// New member operations
	AddGroupMember(ctx context.Context, tripID, userID string) error
}

// ChatServiceImpl struct that implements the ChatService interface
type ChatServiceImpl struct {
	chatStore    store.ChatStore
	tripStore    store.TripStore
	userStore    store.UserStore // Add UserStore dependency
	eventService types.EventPublisher
	log          *zap.SugaredLogger
}

// NewChatService creates a new ChatService instance
func NewChatService(
	chatStore store.ChatStore,
	tripStore store.TripStore,
	eventService types.EventPublisher,
) *ChatServiceImpl {
	return &ChatServiceImpl{
		chatStore:    chatStore,
		tripStore:    tripStore,
		eventService: eventService,
		log:          logger.GetLogger().Named("ChatService"),
	}
}

// SetUserStore sets the UserStore dependency - can be used after construction
func (s *ChatServiceImpl) SetUserStore(userStore store.UserStore) {
	s.userStore = userStore
}

// GetChatGroupMembers retrieves the members of a chat group
func (s *ChatServiceImpl) GetChatGroupMembers(ctx context.Context, groupID string) ([]types.UserResponse, error) {
	// Check if group exists
	group, err := s.chatStore.GetChatGroup(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("error getting chat group: %w", err)
	}

	// Get members
	members, err := s.chatStore.ListChatGroupMembers(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("error listing chat group members: %w", err)
	}

	// If we have no specific member information in the DB,
	// we can fall back to getting the trip members and filtering
	if len(members) == 0 {
		tripMembers, err := s.tripStore.GetTripMembers(ctx, group.TripID)
		if err != nil {
			return nil, fmt.Errorf("error getting trip members: %w", err)
		}

		userResponses := make([]types.UserResponse, 0, len(tripMembers))
		for _, member := range tripMembers {
			// Get user profile if UserStore is available
			if s.userStore != nil {
				user, err := s.userStore.GetUserByID(ctx, member.UserID)
				if err == nil {
					userResponses = append(userResponses, types.UserResponse{
						ID:          user.ID,
						Email:       user.Email,
						Username:    user.Username,
						FirstName:   user.FirstName,
						LastName:    user.LastName,
						AvatarURL:   user.ProfilePictureURL,
						DisplayName: user.GetDisplayName(),
					})
				}
			}
		}
		return userResponses, nil
	}

	return members, nil
}

// CreateChatMessage creates a new chat message
func (s *ChatServiceImpl) CreateChatMessage(ctx context.Context, message types.ChatMessage) (string, error) {
	// Get chat group to determine the trip
	group, err := s.chatStore.GetChatGroup(ctx, message.GroupID)
	if err != nil {
		return "", fmt.Errorf("error getting chat group: %w", err)
	}

	// Verify the user making the request is part of the trip
	userID, ok := ctx.Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		fmt.Printf("CreateChatMessage: No userID in context! (message.UserID: %s)\n", message.UserID)
		return "", errors.New("unauthorized: missing user ID in context")
	}

	fmt.Printf("CreateChatMessage: Found userID in context: %s (message.UserID: %s)\n",
		userID, message.UserID)

	// Create the message
	id, err := s.chatStore.CreateChatMessage(ctx, message)
	if err != nil {
		return "", fmt.Errorf("error creating chat message: %w", err)
	}

	fmt.Printf("CreateChatMessage: Generated message ID: %s\n", id)

	// Get the full message with ID and timestamps
	createdMessage, err := s.chatStore.GetChatMessageByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("error retrieving created message: %w", err)
	}

	fmt.Printf("CreateChatMessage: Retrieved created message with ID: %s\n", createdMessage.ID)

	// Get user details for the event
	var user *types.UserResponse
	if s.userStore != nil {
		userInfo, err := s.userStore.GetUserByID(ctx, message.UserID)
		if err == nil {
			userProfile := types.UserResponse{
				ID:          userInfo.ID,
				Email:       userInfo.Email,
				Username:    userInfo.Username,
				FirstName:   userInfo.FirstName,
				LastName:    userInfo.LastName,
				AvatarURL:   userInfo.ProfilePictureURL,
				DisplayName: userInfo.GetDisplayName(),
			}
			user = &userProfile
		}
	}

	// Publish message create event
	// Use explicit context to ensure user ID is in context
	eventCtx := context.WithValue(ctx, middleware.UserIDKey, message.UserID)
	s.publishChatEvent(eventCtx, group.TripID, "message_created", *createdMessage, user)

	return id, nil
}

// AddGroupMember adds a member to a chat group
// This is needed to implement the models/chat/service.ChatServiceInterface
func (s *ChatServiceImpl) AddGroupMember(ctx context.Context, tripID, userID string) error {
	// Find the group for the trip
	// This is a simplified implementation - in a real system we'd need to
	// make sure we find the right group for the trip
	groups, err := s.chatStore.ListChatGroupsByTrip(ctx, tripID, 10, 0)
	if err != nil {
		return err
	}

	if len(groups.Groups) == 0 {
		return fmt.Errorf("no chat group found for trip %s", tripID)
	}

	// Add the member to the first group
	groupID := groups.Groups[0].ID
	return s.chatStore.AddChatGroupMember(ctx, groupID, userID)
}

// --- Interface Implementation (placeholders for now) ---

func (s *ChatServiceImpl) CreateGroup(ctx context.Context, tripID, groupName, createdByUserID string) (*types.ChatGroup, error) {
	// Verify user can create a group for this trip
	role, err := s.tripStore.GetUserRole(ctx, tripID, createdByUserID)
	if err != nil {
		// Simplified error check: if GetUserRole returns any error, assume user is not a member or not authorized.
		// This can be refined once the custom error structure (apperrors.Error, apperrors.ErrorCodeNotFound) is confirmed.
		var specificError *apperrors.AppError
		if errors.As(err, &specificError) && specificError.Type == apperrors.NotFoundError {
			s.log.Warnw("User not found or not an active member during group creation", "tripID", tripID, "userID", createdByUserID, "error", err)
			return nil, fmt.Errorf("user is not an active member of this trip or trip does not exist")
		}
		s.log.Errorw("Error checking trip membership during group creation", "tripID", tripID, "userID", createdByUserID, "error", err)
		return nil, fmt.Errorf("permission denied or error checking trip membership: %w", err)
	}

	// At this point, err is nil, so 'role' is a valid role (OWNER, MEMBER, or ADMIN).
	// Check if the user's role is sufficient to create a group.
	// For example, allow only ADMIN or OWNER.
	if !(role == types.MemberRoleOwner || role == types.MemberRoleAdmin) { // Example: Only Owner or Admin can create.
		s.log.Warnw("User does not have permission to create group", "tripID", tripID, "userID", createdByUserID, "userRole", role)
		return nil, fmt.Errorf("user (role: %s) does not have sufficient permission to create a group", role)
	}

	// Create the group
	group := types.ChatGroup{
		TripID:    tripID,
		Name:      groupName,
		CreatedBy: createdByUserID,
	}

	groupID, err := s.chatStore.CreateChatGroup(ctx, group)
	if err != nil {
		s.log.Errorw("Failed to create chat group in store", "tripID", tripID, "groupName", groupName, "error", err)
		return nil, fmt.Errorf("error creating chat group: %w", err)
	}

	// Add the creator as a member
	err = s.chatStore.AddChatGroupMember(ctx, groupID, createdByUserID)
	if err != nil {
		s.log.Errorw("Failed to add creator to chat group", "groupID", groupID, "userID", createdByUserID, "error", err)
		// Potentially rollback group creation or handle inconsistency
		return nil, fmt.Errorf("error adding creator to chat group: %w", err)
	}

	// Get the created group
	createdGroup, err := s.chatStore.GetChatGroup(ctx, groupID)
	if err != nil {
		s.log.Errorw("Failed to retrieve created chat group", "groupID", groupID, "error", err)
		return nil, fmt.Errorf("error retrieving created chat group: %w", err)
	}

	s.log.Infow("Chat group created successfully", "groupID", groupID, "tripID", tripID, "groupName", groupName)
	return createdGroup, nil
}

func (s *ChatServiceImpl) GetGroup(ctx context.Context, groupID, requestingUserID string) (*types.ChatGroup, error) {
	// Get the group
	group, err := s.chatStore.GetChatGroup(ctx, groupID)
	if err != nil {
		s.log.Errorw("Failed to get chat group from store", "groupID", groupID, "error", err)
		return nil, fmt.Errorf("error getting chat group: %w", err) // Or apperrors.NotFound if applicable
	}

	// Verify the user making the request is part of the trip
	role, err := s.tripStore.GetUserRole(ctx, group.TripID, requestingUserID)
	if err != nil {
		var specificError *apperrors.AppError
		if errors.As(err, &specificError) && specificError.Type == apperrors.NotFoundError {
			s.log.Warnw("User not found or not an active member during get group", "tripID", group.TripID, "userID", requestingUserID, "error", err)
			return nil, fmt.Errorf("user is not an active member of this trip or trip does not exist")
		}
		s.log.Errorw("Error checking trip membership during get group", "tripID", group.TripID, "userID", requestingUserID, "error", err)
		return nil, fmt.Errorf("permission denied or error checking trip membership: %w", err)
	}

	// Any active member of the trip can view group details.
	// If more specific role checks are needed (e.g. only group members), add them here or in store.
	_ = role // Explicitly use role to satisfy linter if no further checks based on its value are made here.

	s.log.Debugw("User authorized to get group", "groupID", groupID, "userID", requestingUserID, "userRole", role)
	return group, nil
}

func (s *ChatServiceImpl) UpdateGroup(ctx context.Context, groupID, requestingUserID string, updateReq types.ChatGroupUpdateRequest) (*types.ChatGroup, error) {
	// Get the group to find its TripID
	group, err := s.chatStore.GetChatGroup(ctx, groupID)
	if err != nil {
		s.log.Errorw("Failed to get chat group for update", "groupID", groupID, "error", err)
		return nil, fmt.Errorf("error finding group to update: %w", err)
	}

	// Verify the user has permission (e.g., Admin or Owner of the trip)
	role, err := s.tripStore.GetUserRole(ctx, group.TripID, requestingUserID)
	if err != nil {
		var specificError *apperrors.AppError
		if errors.As(err, &specificError) && specificError.Type == apperrors.NotFoundError {
			s.log.Warnw("User not found or not an active member during update group", "tripID", group.TripID, "userID", requestingUserID, "error", err)
			return nil, fmt.Errorf("user is not an active member of this trip or trip does not exist")
		}
		s.log.Errorw("Error checking trip membership during update group", "tripID", group.TripID, "userID", requestingUserID, "error", err)
		return nil, fmt.Errorf("permission denied or error checking trip membership: %w", err)
	}

	if !(role == types.MemberRoleAdmin || role == types.MemberRoleOwner) {
		s.log.Warnw("User does not have permission to update group", "groupID", groupID, "userID", requestingUserID, "userRole", role)
		return nil, fmt.Errorf("user (role: %s) does not have permission to update this group", role)
	}

	// Perform the update
	err = s.chatStore.UpdateChatGroup(ctx, groupID, updateReq)
	if err != nil {
		s.log.Errorw("Failed to update chat group in store", "groupID", groupID, "error", err)
		return nil, fmt.Errorf("error updating chat group: %w", err)
	}

	s.log.Infow("Chat group updated successfully", "groupID", groupID)
	// Fetch and return the updated group
	return s.chatStore.GetChatGroup(ctx, groupID)
}

func (s *ChatServiceImpl) DeleteGroup(ctx context.Context, groupID, requestingUserID string) error {
	// Get the group to check permissions
	group, err := s.chatStore.GetChatGroup(ctx, groupID)
	if err != nil {
		return fmt.Errorf("error getting chat group: %w", err)
	}

	// Verify requesting user is a member of the trip
	role, err := s.tripStore.GetUserRole(ctx, group.TripID, requestingUserID)
	if err != nil {
		return fmt.Errorf("error checking trip membership: %w", err)
	}

	// If GetUserRole returned no error, the user is a member of the trip and 'role' contains their role.
	// The specific role (Admin, Owner, Member) is checked below for authorization.

	// Only admin, owner, or the creator can delete the group
	if role != types.MemberRoleAdmin && role != types.MemberRoleOwner && group.CreatedBy != requestingUserID {
		return fmt.Errorf("not authorized to delete this group")
	}

	// Delete the group
	if err := s.chatStore.DeleteChatGroup(ctx, groupID); err != nil {
		return fmt.Errorf("error deleting chat group: %w", err)
	}

	// Publish event
	data := map[string]interface{}{
		"group_id": groupID,
	}

	s.publishChatEvent(ctx, group.TripID, "group_deleted", data, nil)

	return nil
}

func (s *ChatServiceImpl) ListTripGroups(ctx context.Context, tripID, requestingUserID string, params types.PaginationParams) (*types.ChatGroupPaginatedResponse, error) {
	// Verify user is a member of the trip
	_, err := s.tripStore.GetUserRole(ctx, tripID, requestingUserID)
	if err != nil {
		return nil, fmt.Errorf("error checking trip membership: %w", err)
	}

	// Get the groups from the store
	return s.chatStore.ListChatGroupsByTrip(ctx, tripID, params.Limit, params.Offset)
}

func (s *ChatServiceImpl) AddMember(ctx context.Context, groupID, actorUserID, targetUserID string) error {
	// Get the group to check permissions
	group, err := s.chatStore.GetChatGroup(ctx, groupID)
	if err != nil {
		return fmt.Errorf("error getting chat group: %w", err)
	}

	// Verify actor is a member of the trip with appropriate permissions
	actorRole, err := s.tripStore.GetUserRole(ctx, group.TripID, actorUserID)
	if err != nil {
		return fmt.Errorf("error checking actor's trip membership: %w", err)
	}

	// If GetUserRole returned no error, the actor is a member of the trip.
	// No need for MemberRoleNone check here.

	// Only admins, owners, or the group creator can add members
	if actorRole != types.MemberRoleAdmin && actorRole != types.MemberRoleOwner && group.CreatedBy != actorUserID {
		return fmt.Errorf("not authorized to add members to this group")
	}

	// Verify target user is a member of the trip
	_, err = s.tripStore.GetUserRole(ctx, group.TripID, targetUserID)
	if err != nil {
		return fmt.Errorf("error checking target user's trip membership or target user not a member: %w", err)
	}

	// Add the member
	if err := s.chatStore.AddChatGroupMember(ctx, groupID, targetUserID); err != nil {
		return fmt.Errorf("error adding member to chat group: %w", err)
	}

	// Get user details for the event
	var user *types.UserResponse
	if s.userStore != nil {
		userInfo, err := s.userStore.GetUserByID(ctx, targetUserID)
		if err == nil {
			userProfile := types.UserResponse{
				ID:          userInfo.ID,
				Email:       userInfo.Email,
				Username:    userInfo.Username,
				FirstName:   userInfo.FirstName,
				LastName:    userInfo.LastName,
				AvatarURL:   userInfo.ProfilePictureURL,
				DisplayName: userInfo.GetDisplayName(),
			}
			user = &userProfile
		}
	}

	// Publish event
	data := map[string]interface{}{
		"group_id": groupID,
		"user_id":  targetUserID,
	}

	s.publishChatEvent(ctx, group.TripID, "member_added", data, user)

	return nil
}

func (s *ChatServiceImpl) RemoveMember(ctx context.Context, groupID, actorUserID, targetUserID string) error {
	// Get the group to check permissions
	group, err := s.chatStore.GetChatGroup(ctx, groupID)
	if err != nil {
		return fmt.Errorf("error getting chat group: %w", err)
	}

	// Self-removal is always allowed
	if actorUserID != targetUserID {
		// Verify actor is a member of the trip with appropriate permissions
		actorRole, err := s.tripStore.GetUserRole(ctx, group.TripID, actorUserID)
		if err != nil {
			return fmt.Errorf("error checking actor's trip membership: %w", err)
		}

		// If GetUserRole returned no error, the actor is a member of the trip.
		// No need for MemberRoleNone check here.

		// Only admins, owners, or the group creator can remove members
		if actorRole != types.MemberRoleAdmin && actorRole != types.MemberRoleOwner && group.CreatedBy != actorUserID {
			return fmt.Errorf("not authorized to remove members from this group")
		}
	}

	// Remove the member
	if err := s.chatStore.RemoveChatGroupMember(ctx, groupID, targetUserID); err != nil {
		return fmt.Errorf("error removing member from chat group: %w", err)
	}

	// Publish event
	data := map[string]interface{}{
		"group_id": groupID,
		"user_id":  targetUserID,
	}

	s.publishChatEvent(ctx, group.TripID, "member_removed", data, nil)

	return nil
}

func (s *ChatServiceImpl) ListMembers(ctx context.Context, groupID, requestingUserID string) ([]types.UserResponse, error) {
	// Get the group to check permissions
	group, err := s.chatStore.GetChatGroup(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("error getting chat group: %w", err)
	}

	// Verify requesting user is a member of the trip
	_, err = s.tripStore.GetUserRole(ctx, group.TripID, requestingUserID)
	if err != nil {
		return nil, fmt.Errorf("error checking trip membership: %w", err)
	}

	// Get the members
	return s.chatStore.ListChatGroupMembers(ctx, groupID)
}

func (s *ChatServiceImpl) PostMessage(ctx context.Context, groupID, userID, content string) (*types.ChatMessageWithUser, error) {
	// Debug context
	if ctxUserID, ok := ctx.Value(middleware.UserIDKey).(string); ok && ctxUserID != "" {
		fmt.Printf("PostMessage: Found userID in context: %s (param userID: %s)\n",
			ctxUserID, userID)
	} else {
		fmt.Printf("PostMessage: No userID in context! (param userID: %s)\n", userID)
	}

	// Verify the user is a member of the trip that owns this group
	group, err := s.chatStore.GetChatGroup(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("error retrieving chat group: %w", err)
	}

	_, err = s.tripStore.GetUserRole(ctx, group.TripID, userID)
	if err != nil {
		return nil, fmt.Errorf("error checking trip membership: %w", err)
	}

	// Create the message
	message := types.ChatMessage{
		GroupID:     groupID,
		UserID:      userID,
		Content:     content,
		ContentType: types.ContentTypeText,
		TripID:      group.TripID, // Set the trip ID based on the group
	}

	messageID, err := s.chatStore.CreateChatMessage(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}

	// Get the full message with ID and timestamps
	createdMessage, err := s.chatStore.GetChatMessageByID(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("error retrieving created message: %w", err)
	}

	// Get user info
	var userInfo types.UserResponse
	if s.userStore != nil {
		user, err := s.userStore.GetUserByID(ctx, userID)
		if err == nil {
			userInfo = types.UserResponse{
				ID:          user.ID,
				Email:       user.Email,
				Username:    user.Username,
				FirstName:   user.FirstName,
				LastName:    user.LastName,
				AvatarURL:   user.ProfilePictureURL,
				DisplayName: user.GetDisplayName(),
			}
		}
	}

	result := &types.ChatMessageWithUser{
		Message: *createdMessage,
		User:    userInfo,
	}

	// Ensure context has userID before publishing event
	// Use explicit context to ensure user ID is in context
	eventCtx := context.WithValue(ctx, middleware.UserIDKey, userID)
	fmt.Printf("PostMessage: Using explicit eventCtx with userID: %s for publishing\n", userID)

	// Publish message event
	s.publishChatEvent(eventCtx, group.TripID, "message_created", *createdMessage, &userInfo)

	return result, nil
}

// GetMessage gets a message by ID
func (s *ChatServiceImpl) GetMessage(ctx context.Context, messageID, requestingUserID string) (*types.ChatMessageWithUser, error) {
	// Get the message
	message, err := s.chatStore.GetChatMessageByID(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("error getting message: %w", err)
	}

	// Get the group to find the trip and check permissions
	group, err := s.chatStore.GetChatGroup(ctx, message.GroupID)
	if err != nil {
		return nil, fmt.Errorf("error getting chat group: %w", err)
	}

	// Verify requesting user is a member of the trip
	_, err = s.tripStore.GetUserRole(ctx, group.TripID, requestingUserID)
	if err != nil {
		return nil, fmt.Errorf("error checking trip membership: %w", err)
	}

	// Get user info
	var userInfo types.UserResponse
	if s.userStore != nil {
		user, err := s.userStore.GetUserByID(ctx, message.UserID)
		if err == nil {
			userInfo = types.UserResponse{
				ID:          user.ID,
				Email:       user.Email,
				Username:    user.Username,
				FirstName:   user.FirstName,
				LastName:    user.LastName,
				AvatarURL:   user.ProfilePictureURL,
				DisplayName: user.GetDisplayName(),
			}
		}
	}

	return &types.ChatMessageWithUser{
		Message: *message,
		User:    userInfo,
	}, nil
}

func (s *ChatServiceImpl) UpdateMessage(ctx context.Context, messageID, requestingUserID, newContent string) (*types.ChatMessageWithUser, error) {
	// Get message to check permissions
	message, err := s.chatStore.GetChatMessageByID(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("error getting message: %w", err)
	}

	// Check if the user is the message author or has admin rights
	if message.UserID != requestingUserID {
		// Get the group to find the trip
		group, err := s.chatStore.GetChatGroup(ctx, message.GroupID)
		if err != nil {
			return nil, fmt.Errorf("error getting chat group: %w", err)
		}

		// Check if user is a trip admin
		role, err := s.tripStore.GetUserRole(ctx, group.TripID, requestingUserID)
		if err != nil {
			return nil, fmt.Errorf("error checking trip membership: %w", err)
		}

		if role != types.MemberRoleAdmin && role != types.MemberRoleOwner {
			return nil, fmt.Errorf("not authorized to update this message")
		}
	}

	// Update the message
	if err := s.chatStore.UpdateChatMessage(ctx, messageID, newContent); err != nil {
		return nil, fmt.Errorf("error updating message: %w", err)
	}

	// Get the updated message
	updatedMessage, err := s.chatStore.GetChatMessageByID(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("error getting updated message: %w", err)
	}

	// Get the group to find the trip (for event publishing)
	group, err := s.chatStore.GetChatGroup(ctx, updatedMessage.GroupID)
	if err != nil {
		return nil, fmt.Errorf("error getting chat group: %w", err)
	}

	// Get user info for the response
	var userInfo types.UserResponse
	if s.userStore != nil {
		user, err := s.userStore.GetUserByID(ctx, updatedMessage.UserID)
		if err == nil {
			userInfo = types.UserResponse{
				ID:          user.ID,
				Email:       user.Email,
				Username:    user.Username,
				FirstName:   user.FirstName,
				LastName:    user.LastName,
				AvatarURL:   user.ProfilePictureURL,
				DisplayName: user.GetDisplayName(),
			}
		}
	}

	result := &types.ChatMessageWithUser{
		Message: *updatedMessage,
		User:    userInfo,
	}

	// Publish update event
	s.publishChatEvent(ctx, group.TripID, "message_updated", updatedMessage, &userInfo)

	return result, nil
}

func (s *ChatServiceImpl) DeleteMessage(ctx context.Context, messageID, requestingUserID string) error {
	// Get message to check permissions
	message, err := s.chatStore.GetChatMessageByID(ctx, messageID)
	if err != nil {
		return fmt.Errorf("error getting message: %w", err)
	}

	// Check if the user is the message author or has admin rights
	if message.UserID != requestingUserID {
		// Get the group to find the trip
		group, err := s.chatStore.GetChatGroup(ctx, message.GroupID)
		if err != nil {
			return fmt.Errorf("error getting chat group: %w", err)
		}

		// Check if user is a trip admin
		role, err := s.tripStore.GetUserRole(ctx, group.TripID, requestingUserID)
		if err != nil {
			return fmt.Errorf("error checking trip membership: %w", err)
		}

		if role != types.MemberRoleAdmin && role != types.MemberRoleOwner {
			return fmt.Errorf("not authorized to delete this message")
		}
	}

	// Get the group for event publishing
	group, err := s.chatStore.GetChatGroup(ctx, message.GroupID)
	if err != nil {
		return fmt.Errorf("error getting chat group: %w", err)
	}

	// Delete the message
	if err := s.chatStore.DeleteChatMessage(ctx, messageID); err != nil {
		return fmt.Errorf("error deleting message: %w", err)
	}

	// Publish delete event
	data := map[string]interface{}{
		"message_id": messageID,
		"group_id":   message.GroupID,
	}

	s.publishChatEvent(ctx, group.TripID, "message_deleted", data, nil)

	return nil
}

func (s *ChatServiceImpl) ListMessages(ctx context.Context, groupID, requestingUserID string, params types.PaginationParams) (*types.ChatMessagePaginatedResponse, error) {
	// Verify the user is a member of the trip that owns this group
	group, err := s.chatStore.GetChatGroup(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("error retrieving chat group: %w", err)
	}

	_, err = s.tripStore.GetUserRole(ctx, group.TripID, requestingUserID)
	if err != nil {
		return nil, fmt.Errorf("error checking trip membership: %w", err)
	}

	// Get messages from the store
	messages, total, err := s.chatStore.ListChatMessages(ctx, groupID, params)
	if err != nil {
		return nil, fmt.Errorf("error listing messages: %w", err)
	}

	// Convert messages to include user info
	messageWithUsers := make([]types.ChatMessageWithUser, len(messages))
	for i, msg := range messages {
		var userInfo types.UserResponse
		if s.userStore != nil {
			user, err := s.userStore.GetUserByID(ctx, msg.UserID)
			if err == nil {
				userInfo = types.UserResponse{
					ID:          user.ID,
					Email:       user.Email,
					Username:    user.Username,
					FirstName:   user.FirstName,
					LastName:    user.LastName,
					AvatarURL:   user.ProfilePictureURL,
					DisplayName: user.GetDisplayName(),
				}
			}
		}

		messageWithUsers[i] = types.ChatMessageWithUser{
			Message: msg,
			User:    userInfo,
		}
	}

	return &types.ChatMessagePaginatedResponse{
		Messages: messageWithUsers,
		Total:    total,
		Limit:    params.Limit,
		Offset:   params.Offset,
	}, nil
}

func (s *ChatServiceImpl) AddReaction(ctx context.Context, messageID, userID, reaction string) error {
	// Get message to check permissions
	message, err := s.chatStore.GetChatMessageByID(ctx, messageID)
	if err != nil {
		return fmt.Errorf("error getting message: %w", err)
	}

	// Get the group to find the trip
	group, err := s.chatStore.GetChatGroup(ctx, message.GroupID)
	if err != nil {
		return fmt.Errorf("error getting chat group: %w", err)
	}

	// Verify user is a member of the trip
	_, err = s.tripStore.GetUserRole(ctx, group.TripID, userID)
	if err != nil {
		return fmt.Errorf("error checking trip membership: %w", err)
	}

	// Add the reaction
	if err := s.chatStore.AddReaction(ctx, messageID, userID, reaction); err != nil {
		return fmt.Errorf("error adding reaction: %w", err)
	}

	// Publish event
	data := map[string]interface{}{
		"message_id": messageID,
		"reaction":   reaction,
	}

	// Get user details for the event
	var user *types.UserResponse
	if s.userStore != nil {
		userInfo, err := s.userStore.GetUserByID(ctx, userID)
		if err == nil {
			userProfile := types.UserResponse{
				ID:          userInfo.ID,
				Email:       userInfo.Email,
				Username:    userInfo.Username,
				FirstName:   userInfo.FirstName,
				LastName:    userInfo.LastName,
				AvatarURL:   userInfo.ProfilePictureURL,
				DisplayName: userInfo.GetDisplayName(),
			}
			user = &userProfile
		}
	}

	s.publishChatEvent(ctx, group.TripID, "reaction_added", data, user)

	return nil
}

func (s *ChatServiceImpl) RemoveReaction(ctx context.Context, messageID, userID, reaction string) error {
	// Get message to check permissions
	message, err := s.chatStore.GetChatMessageByID(ctx, messageID)
	if err != nil {
		return fmt.Errorf("error getting message: %w", err)
	}

	// Get the group to find the trip
	group, err := s.chatStore.GetChatGroup(ctx, message.GroupID)
	if err != nil {
		return fmt.Errorf("error getting chat group: %w", err)
	}

	// Verify user is a member of the trip
	_, err = s.tripStore.GetUserRole(ctx, group.TripID, userID)
	if err != nil {
		return fmt.Errorf("error checking trip membership: %w", err)
	}

	// Remove the reaction
	if err := s.chatStore.RemoveReaction(ctx, messageID, userID, reaction); err != nil {
		return fmt.Errorf("error removing reaction: %w", err)
	}

	// Publish event
	data := map[string]interface{}{
		"message_id": messageID,
		"reaction":   reaction,
	}

	s.publishChatEvent(ctx, group.TripID, "reaction_removed", data, nil)

	return nil
}

func (s *ChatServiceImpl) UpdateLastRead(ctx context.Context, groupID, userID, messageID string) error {
	// Verify the user is a member of the trip that owns this group
	group, err := s.chatStore.GetChatGroup(ctx, groupID)
	if err != nil {
		return fmt.Errorf("error retrieving chat group: %w", err)
	}

	_, err = s.tripStore.GetUserRole(ctx, group.TripID, userID)
	if err != nil {
		return fmt.Errorf("error checking trip membership: %w", err)
	}

	// Update the last read status
	if err := s.chatStore.UpdateLastReadMessage(ctx, groupID, userID, messageID); err != nil {
		return fmt.Errorf("error updating last read message: %w", err)
	}

	// Publish event for last read updated
	data := map[string]interface{}{
		"message_id": messageID,
		"group_id":   groupID,
	}

	s.publishChatEvent(ctx, group.TripID, "last_read_updated", data, nil)

	return nil
}

// publishChatEvent publishes a chat-related event
func (s *ChatServiceImpl) publishChatEvent(ctx context.Context, tripID, eventType string, data interface{}, user *types.UserResponse) {
	if s.eventService == nil {
		fmt.Printf("publishChatEvent: eventService is nil! Cannot publish event\n")
		return
	}

	// Get user ID from context
	userID, ok := ctx.Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		// Log but don't fail
		fmt.Printf("publishChatEvent: userID not found in context. Type of value: %T\n",
			ctx.Value(middleware.UserIDKey))
		s.log.Warnw("Failed to get user ID from context for event publishing", "eventType", eventType)
		return
	}

	fmt.Printf("publishChatEvent: Successfully retrieved userID from context: %s for event: %s\n",
		userID, eventType)

	// Convert data to JSON for payload
	eventData := map[string]interface{}{
		"data": data,
	}
	if user != nil {
		eventData["user"] = user
	}

	payload, err := json.Marshal(eventData)
	if err != nil {
		s.log.Errorw("Failed to marshal event data", "error", err)
		return
	}

	// Create the proper event type with the "chat." prefix
	chatEventType := "chat." + eventType

	// Create event
	event := types.Event{
		BaseEvent: types.BaseEvent{
			ID:        utils.GenerateEventID(),
			Type:      types.EventType(chatEventType),
			TripID:    tripID,
			UserID:    userID,
			Timestamp: time.Now(),
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source: "chat-service",
		},
		Payload: payload,
	}

	// Publish the event
	if err := s.eventService.Publish(ctx, tripID, event); err != nil {
		s.log.Errorw("Failed to publish chat event", "error", err, "eventType", chatEventType)
	}
}
