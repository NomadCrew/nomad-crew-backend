package models

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/internal/utils"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

type PollModel struct {
	store          store.PollStore
	tripModel      TripModelInterface
	eventPublisher EventPublisherInterface
}

func NewPollModel(store store.PollStore, tripModel TripModelInterface, eventPublisher EventPublisherInterface) *PollModel {
	return &PollModel{
		store:          store,
		tripModel:      tripModel,
		eventPublisher: eventPublisher,
	}
}

// CreatePollWithEvent creates a poll with options and publishes a creation event.
func (pm *PollModel) CreatePollWithEvent(ctx context.Context, tripID, userID string, req *types.PollCreate) (*types.PollResponse, error) {
	log := logger.GetLogger()

	// Validate input
	if err := validatePollCreate(req); err != nil {
		return nil, err
	}

	// Verify trip membership
	if err := pm.verifyTripMembership(ctx, tripID, userID); err != nil {
		return nil, err
	}

	// Create the poll and options atomically
	poll := &types.Poll{
		TripID:             tripID,
		Question:           req.Question,
		PollType:           req.PollType,
		IsBlind:            req.IsBlind,
		AllowMultipleVotes: req.AllowMultipleVotes,
		CreatedBy:          userID,
	}

	// Calculate expiration
	durationMins := 1440 // default: 24 hours
	if req.DurationMinutes != nil {
		durationMins = *req.DurationMinutes
	}
	poll.ExpiresAt = time.Now().Add(time.Duration(durationMins) * time.Minute)

	// Build options — prefer RichOptions if provided, fall back to simple Options
	var options []*types.PollOption
	if len(req.RichOptions) > 0 {
		options = make([]*types.PollOption, 0, len(req.RichOptions))
		for i, ro := range req.RichOptions {
			opt := &types.PollOption{
				Text:      strings.TrimSpace(ro.Text),
				Position:  i,
				CreatedBy: userID,
			}
			if ro.Metadata != nil {
				opt.OptionMetadata = ro.Metadata
				opt.ImageURL = ro.Metadata.ImageURL
				opt.Lat = ro.Metadata.Lat
				opt.Lng = ro.Metadata.Lng
			}
			options = append(options, opt)
		}
	} else {
		options = make([]*types.PollOption, 0, len(req.Options))
		for i, optText := range req.Options {
			options = append(options, &types.PollOption{
				Text:      strings.TrimSpace(optText),
				Position:  i,
				CreatedBy: userID,
			})
		}
	}

	pollID, err := pm.store.CreatePollWithOptions(ctx, poll, options)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	// Fetch the created poll to get server-generated fields (timestamps, status)
	createdPoll, err := pm.store.GetPoll(ctx, pollID, tripID)
	if err != nil {
		return nil, err
	}

	// Build response
	resp, err := pm.buildPollResponse(ctx, createdPoll, userID)
	if err != nil {
		return nil, err
	}

	// Publish event (fire-and-forget)
	payload := map[string]interface{}{
		"pollId":   pollID,
		"tripId":   tripID,
		"question": req.Question,
	}
	if err := pm.publishPollEvent(ctx, types.EventTypePollCreated, tripID, userID, payload); err != nil {
		log.Warnw("Failed to publish poll created event", "error", err, "pollId", pollID)
	}

	return resp, nil
}

// GetPollWithResults retrieves a poll with full vote data.
func (pm *PollModel) GetPollWithResults(ctx context.Context, tripID, pollID, userID string) (*types.PollResponse, error) {
	// Verify trip membership
	if err := pm.verifyTripMembership(ctx, tripID, userID); err != nil {
		return nil, err
	}

	poll, err := pm.store.GetPoll(ctx, pollID, tripID)
	if err != nil {
		return nil, err
	}

	return pm.buildPollResponse(ctx, poll, userID)
}

// ListTripPolls returns paginated polls for a trip.
func (pm *PollModel) ListTripPolls(ctx context.Context, tripID, userID string, limit, offset int) ([]*types.PollResponse, int, error) {
	// Verify trip membership
	if err := pm.verifyTripMembership(ctx, tripID, userID); err != nil {
		return nil, 0, err
	}

	polls, total, err := pm.store.ListPolls(ctx, tripID, limit, offset)
	if err != nil {
		return nil, 0, errors.NewDatabaseError(err)
	}

	responses := make([]*types.PollResponse, 0, len(polls))
	for _, poll := range polls {
		resp, err := pm.buildPollResponse(ctx, poll, userID)
		if err != nil {
			return nil, 0, err
		}
		responses = append(responses, resp)
	}

	return responses, total, nil
}

// UpdatePollWithEvent updates a poll's question and publishes an event.
func (pm *PollModel) UpdatePollWithEvent(ctx context.Context, tripID, pollID, userID string, req *types.PollUpdate) (*types.PollResponse, error) {
	log := logger.GetLogger()

	// Get the poll to check ownership
	poll, err := pm.store.GetPoll(ctx, pollID, tripID)
	if err != nil {
		return nil, err
	}

	// Check permission: ADMIN+ or creator
	isOwner := poll.CreatedBy == userID
	role, err := pm.tripModel.GetUserRole(ctx, tripID, userID)
	if err != nil {
		return nil, errors.Forbidden("unauthorized", "user is not a member of this trip")
	}

	if !types.CanPerformWithOwnership(role, types.ActionUpdate, types.ResourcePoll, isOwner) {
		return nil, errors.Forbidden("unauthorized", "you don't have permission to update this poll")
	}

	// Validate update
	if req.Question == nil || *req.Question == "" {
		return nil, errors.ValidationFailed("invalid_update", "question is required")
	}
	if len(*req.Question) > 500 {
		return nil, errors.ValidationFailed("invalid_update", "question exceeds 500 characters")
	}

	// Check if poll has any votes - reject update if it does
	votes, err := pm.store.ListVotesByPoll(ctx, pollID)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	if len(votes) > 0 {
		return nil, errors.ValidationFailed("poll_has_votes", "cannot update a poll that already has votes")
	}

	// Update the question (returns the updated poll)
	updatedPoll, err := pm.store.UpdatePollQuestion(ctx, pollID, tripID, *req.Question)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	// Build response using the already-fetched updated poll
	resp, err := pm.buildPollResponse(ctx, updatedPoll, userID)
	if err != nil {
		return nil, err
	}

	// Publish event (fire-and-forget)
	payload := map[string]interface{}{
		"pollId":   pollID,
		"tripId":   tripID,
		"question": *req.Question,
	}
	if err := pm.publishPollEvent(ctx, types.EventTypePollUpdated, tripID, userID, payload); err != nil {
		log.Warnw("Failed to publish poll updated event", "error", err, "pollId", pollID)
	}

	return resp, nil
}

// CastVoteWithEvent casts a vote and publishes an event.
func (pm *PollModel) CastVoteWithEvent(ctx context.Context, tripID, pollID, optionID, userID string) error {
	log := logger.GetLogger()

	// Verify trip membership
	if err := pm.verifyTripMembership(ctx, tripID, userID); err != nil {
		return err
	}

	// Get poll (with tripID check for IDOR prevention)
	poll, err := pm.store.GetPoll(ctx, pollID, tripID)
	if err != nil {
		return err
	}

	// Check poll is active
	if poll.Status != types.PollStatusActive {
		return errors.ValidationFailed("poll_closed", "cannot vote on a closed poll")
	}

	// Check poll is not expired
	if poll.IsExpired() {
		return errors.ValidationFailed("poll_expired", "cannot vote on an expired poll")
	}

	// Validate optionID belongs to this poll
	options, err := pm.store.ListPollOptions(ctx, pollID)
	if err != nil {
		return errors.NewDatabaseError(err)
	}

	validOption := false
	for _, opt := range options {
		if opt.ID == optionID {
			validOption = true
			break
		}
	}
	if !validOption {
		return errors.NotFound("option", optionID)
	}

	// Single-choice: atomic swap (delete all user votes + insert new one in a single tx)
	if !poll.AllowMultipleVotes {
		if err := pm.store.SwapVote(ctx, pollID, optionID, userID); err != nil {
			return errors.NewDatabaseError(err)
		}
	} else {
		// Multi-choice: just cast vote (ON CONFLICT DO NOTHING handles dedup)
		if err := pm.store.CastVote(ctx, pollID, optionID, userID); err != nil {
			return errors.NewDatabaseError(err)
		}
	}

	// Publish event (fire-and-forget)
	payload := map[string]interface{}{
		"pollId":   pollID,
		"tripId":   tripID,
		"optionId": optionID,
		"userId":   userID,
	}
	if err := pm.publishPollEvent(ctx, types.EventTypePollVoteCast, tripID, userID, payload); err != nil {
		log.Warnw("Failed to publish vote cast event", "error", err, "pollId", pollID)
	}

	return nil
}

// RemoveVoteWithEvent removes a vote and publishes an event.
func (pm *PollModel) RemoveVoteWithEvent(ctx context.Context, tripID, pollID, optionID, userID string) error {
	log := logger.GetLogger()

	// Verify trip membership
	if err := pm.verifyTripMembership(ctx, tripID, userID); err != nil {
		return err
	}

	// Get poll (with tripID check for IDOR prevention)
	poll, err := pm.store.GetPoll(ctx, pollID, tripID)
	if err != nil {
		return err
	}

	// Check poll is active
	if poll.Status != types.PollStatusActive {
		return errors.ValidationFailed("poll_closed", "cannot modify votes on a closed poll")
	}

	// Remove the vote
	if err := pm.store.RemoveVote(ctx, pollID, optionID, userID); err != nil {
		return errors.NewDatabaseError(err)
	}

	// Publish event (fire-and-forget)
	payload := map[string]interface{}{
		"pollId":   pollID,
		"tripId":   tripID,
		"optionId": optionID,
	}
	if err := pm.publishPollEvent(ctx, types.EventTypePollVoteRemoved, tripID, userID, payload); err != nil {
		log.Warnw("Failed to publish vote removed event", "error", err, "pollId", pollID)
	}

	return nil
}

// ClosePollWithEvent closes a poll and publishes an event.
func (pm *PollModel) ClosePollWithEvent(ctx context.Context, tripID, pollID, userID string) (*types.PollResponse, error) {
	log := logger.GetLogger()

	// Get the poll to check ownership
	poll, err := pm.store.GetPoll(ctx, pollID, tripID)
	if err != nil {
		return nil, err
	}

	// Check permission: ADMIN+ or creator
	isOwner := poll.CreatedBy == userID
	role, err := pm.tripModel.GetUserRole(ctx, tripID, userID)
	if err != nil {
		return nil, errors.Forbidden("unauthorized", "user is not a member of this trip")
	}

	if !types.CanPerformWithOwnership(role, types.ActionUpdate, types.ResourcePoll, isOwner) {
		return nil, errors.Forbidden("unauthorized", "you don't have permission to close this poll")
	}

	// Check close conditions: poll must be expired OR all members must have voted
	if !poll.IsExpired() {
		// Count unique voters
		votes, err := pm.store.ListVotesByPoll(ctx, pollID)
		if err != nil {
			return nil, errors.NewDatabaseError(err)
		}
		uniqueVoters := make(map[string]bool)
		for _, v := range votes {
			uniqueVoters[v.UserID] = true
		}

		// Count active trip members
		members, err := pm.tripModel.GetTripMembers(ctx, tripID)
		if err != nil {
			return nil, fmt.Errorf("failed to get trip members: %w", err)
		}

		if len(uniqueVoters) < len(members) {
			return nil, errors.ValidationFailed("poll_close_restricted",
				"poll cannot be closed until all members have voted or the poll expires")
		}
	}

	// Close the poll (returns the updated poll)
	closedPoll, err := pm.store.ClosePoll(ctx, pollID, tripID, userID)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	// Build response using the already-fetched closed poll
	resp, err := pm.buildPollResponse(ctx, closedPoll, userID)
	if err != nil {
		return nil, err
	}

	// Publish close event (fire-and-forget)
	payload := map[string]interface{}{
		"pollId":   pollID,
		"tripId":   tripID,
		"closedBy": userID,
	}
	if err := pm.publishPollEvent(ctx, types.EventTypePollClosed, tripID, userID, payload); err != nil {
		log.Warnw("Failed to publish poll closed event", "error", err, "pollId", pollID)
	}

	// For blind polls, also fire a POLL_REVEALED event so clients know to show results
	if poll.IsBlind {
		revealPayload := map[string]interface{}{
			"pollId":   pollID,
			"tripId":   tripID,
			"closedBy": userID,
		}
		if err := pm.publishPollEvent(ctx, types.EventTypePollRevealed, tripID, userID, revealPayload); err != nil {
			log.Warnw("Failed to publish poll revealed event", "error", err, "pollId", pollID)
		}
	}

	return resp, nil
}

// DeletePollWithEvent soft-deletes a poll and publishes an event.
func (pm *PollModel) DeletePollWithEvent(ctx context.Context, tripID, pollID, userID string) error {
	log := logger.GetLogger()

	// Get the poll to check ownership
	poll, err := pm.store.GetPoll(ctx, pollID, tripID)
	if err != nil {
		return err
	}

	// Check permission: ADMIN+ or creator
	isOwner := poll.CreatedBy == userID
	role, err := pm.tripModel.GetUserRole(ctx, tripID, userID)
	if err != nil {
		return errors.Forbidden("unauthorized", "user is not a member of this trip")
	}

	if !types.CanPerformWithOwnership(role, types.ActionDelete, types.ResourcePoll, isOwner) {
		return errors.Forbidden("unauthorized", "you don't have permission to delete this poll")
	}

	// Soft delete
	if err := pm.store.SoftDeletePoll(ctx, pollID, tripID); err != nil {
		return errors.NewDatabaseError(err)
	}

	// Publish event (fire-and-forget)
	payload := map[string]interface{}{
		"pollId": pollID,
		"tripId": tripID,
	}
	if err := pm.publishPollEvent(ctx, types.EventTypePollDeleted, tripID, userID, payload); err != nil {
		log.Warnw("Failed to publish poll deleted event", "error", err, "pollId", pollID)
	}

	return nil
}

// buildPollResponse constructs the full PollResponse with vote data.
// If poll is nil, it will be fetched from the store.
func (pm *PollModel) buildPollResponse(ctx context.Context, poll *types.Poll, userID string) (*types.PollResponse, error) {
	pollID := poll.ID

	// Get options
	options, err := pm.store.ListPollOptions(ctx, pollID)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	// Get vote counts
	voteCounts, err := pm.store.GetVoteCountsByPoll(ctx, pollID)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	// Get all votes for voter lists
	allVotes, err := pm.store.ListVotesByPoll(ctx, pollID)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	// Get user's votes
	userVotes, err := pm.store.GetUserVotesForPoll(ctx, pollID, userID)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	// Build user vote set for quick lookup
	userVoteSet := make(map[string]bool)
	for _, v := range userVotes {
		userVoteSet[v.OptionID] = true
	}

	// Build vote map: optionID -> []PollVoter
	voterMap := make(map[string][]types.PollVoter)
	for _, v := range allVotes {
		voterMap[v.OptionID] = append(voterMap[v.OptionID], types.PollVoter{
			UserID:    v.UserID,
			CreatedAt: v.CreatedAt,
		})
	}

	// Build option responses
	totalVotes := 0
	optionResponses := make([]types.PollOptionWithVotes, 0, len(options))
	for _, opt := range options {
		count := voteCounts[opt.ID]
		totalVotes += count

		voters := voterMap[opt.ID]
		if voters == nil {
			voters = []types.PollVoter{}
		}

		optionResponses = append(optionResponses, types.PollOptionWithVotes{
			PollOption: *opt,
			VoteCount:  count,
			Voters:     voters,
			HasVoted:   userVoteSet[opt.ID],
		})
	}

	resp := &types.PollResponse{
		Poll:          *poll,
		Options:       optionResponses,
		TotalVotes:    totalVotes,
		UserVoteCount: len(userVotes),
	}

	// Blind poll: strip vote counts and voter lists while poll is active.
	// Only preserve HasVoted so the user knows they voted.
	if poll.IsBlind && poll.Status == types.PollStatusActive {
		for i := range resp.Options {
			resp.Options[i].VoteCount = 0
			resp.Options[i].Voters = []types.PollVoter{}
		}
		resp.TotalVotes = 0
	}

	return resp, nil
}

// Helper functions

func validatePollCreate(req *types.PollCreate) error {
	var validationErrors []string

	if req.Question == "" {
		validationErrors = append(validationErrors, "question is required")
	}
	if len(req.Question) > 500 {
		validationErrors = append(validationErrors, "question exceeds 500 characters")
	}

	// Default poll type
	if req.PollType == "" {
		req.PollType = types.PollTypeStandard
	}

	// Validate poll type
	switch req.PollType {
	case types.PollTypeStandard, types.PollTypeBinary, types.PollTypeEmoji,
		types.PollTypeSchedule, types.PollTypeVibeCheck:
		// valid
	default:
		validationErrors = append(validationErrors, fmt.Sprintf("invalid poll type: %s", req.PollType))
	}

	// Determine option count from whichever field is populated
	optionCount := len(req.Options)
	if len(req.RichOptions) > 0 {
		optionCount = len(req.RichOptions)
	}

	if optionCount < 2 {
		validationErrors = append(validationErrors, "at least 2 options are required")
	}
	if optionCount > 20 {
		validationErrors = append(validationErrors, "maximum 20 options allowed")
	}

	// Binary polls must have exactly 2 options
	if req.PollType == types.PollTypeBinary && optionCount != 2 {
		validationErrors = append(validationErrors, "binary polls must have exactly 2 options")
	}

	if req.DurationMinutes != nil {
		if *req.DurationMinutes < 5 || *req.DurationMinutes > 2880 {
			validationErrors = append(validationErrors, "durationMinutes must be between 5 and 2880 (5 minutes to 48 hours)")
		}
	}

	// Validate simple options
	if len(req.Options) > 0 {
		seen := make(map[string]bool, len(req.Options))
		for i, opt := range req.Options {
			trimmed := strings.TrimSpace(opt)
			if trimmed == "" {
				validationErrors = append(validationErrors, fmt.Sprintf("option %d cannot be empty", i+1))
			}
			if len(opt) > 200 {
				validationErrors = append(validationErrors, fmt.Sprintf("option %d exceeds 200 characters", i+1))
			}
			// Skip duplicate check for emoji polls (same emoji text is valid)
			if req.PollType != types.PollTypeEmoji {
				lower := strings.ToLower(trimmed)
				if lower != "" && seen[lower] {
					validationErrors = append(validationErrors, fmt.Sprintf("option %d is a duplicate", i+1))
				}
				seen[lower] = true
			}
		}
	}

	// Validate rich options
	if len(req.RichOptions) > 0 {
		seen := make(map[string]bool, len(req.RichOptions))
		for i, opt := range req.RichOptions {
			trimmed := strings.TrimSpace(opt.Text)
			if trimmed == "" {
				validationErrors = append(validationErrors, fmt.Sprintf("option %d cannot be empty", i+1))
			}
			if len(opt.Text) > 200 {
				validationErrors = append(validationErrors, fmt.Sprintf("option %d exceeds 200 characters", i+1))
			}
			if req.PollType != types.PollTypeEmoji {
				lower := strings.ToLower(trimmed)
				if lower != "" && seen[lower] {
					validationErrors = append(validationErrors, fmt.Sprintf("option %d is a duplicate", i+1))
				}
				seen[lower] = true
			}
		}
	}

	if len(validationErrors) > 0 {
		return errors.ValidationFailed(
			"Invalid poll data",
			strings.Join(validationErrors, "; "),
		)
	}

	return nil
}

func (pm *PollModel) verifyTripMembership(ctx context.Context, tripID, userID string) error {
	role, err := pm.tripModel.GetUserRole(ctx, tripID, userID)
	if err != nil {
		return err
	}
	if role == "" {
		return errors.Forbidden(
			"unauthorized",
			"user is not a trip member",
		)
	}
	return nil
}

func (pm *PollModel) publishPollEvent(ctx context.Context, eventType types.EventType, tripID, userID string, payload map[string]interface{}) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	event := types.Event{
		BaseEvent: types.BaseEvent{
			ID:        utils.GenerateEventID(),
			Type:      eventType,
			TripID:    tripID,
			UserID:    userID,
			Timestamp: time.Now(),
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source: "poll_model",
		},
		Payload: payloadJSON,
	}

	return pm.eventPublisher.Publish(ctx, tripID, event)
}
