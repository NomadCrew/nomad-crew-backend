package handlers

import (
	"context"
	"net/http"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PollServiceInterface defines the methods used by PollHandler,
// allowing the handler to be tested with a mock.
type PollServiceInterface interface {
	CreatePollWithEvent(ctx context.Context, tripID, userID string, req *types.PollCreate) (*types.PollResponse, error)
	GetPollWithResults(ctx context.Context, tripID, pollID, userID string) (*types.PollResponse, error)
	ListTripPolls(ctx context.Context, tripID, userID string, limit, offset int) ([]*types.PollResponse, int, error)
	UpdatePollWithEvent(ctx context.Context, tripID, pollID, userID string, req *types.PollUpdate) (*types.PollResponse, error)
	DeletePollWithEvent(ctx context.Context, tripID, pollID, userID string) error
	CastVoteWithEvent(ctx context.Context, tripID, pollID, optionID, userID string) error
	RemoveVoteWithEvent(ctx context.Context, tripID, pollID, optionID, userID string) error
	ClosePollWithEvent(ctx context.Context, tripID, pollID, userID string) (*types.PollResponse, error)
}

// compile-time check: *models.PollModel satisfies PollServiceInterface
var _ PollServiceInterface = (*models.PollModel)(nil)

type PollHandler struct {
	pollService PollServiceInterface
}

func NewPollHandler(service PollServiceInterface) *PollHandler {
	return &PollHandler{
		pollService: service,
	}
}

// isValidUUID validates that a string is a valid UUID
func isValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// CreatePollHandler creates a new poll for a trip.
func (h *PollHandler) CreatePollHandler(c *gin.Context) {
	tripID := c.Param("id")
	if tripID == "" || !isValidUUID(tripID) {
		_ = c.Error(errors.ValidationFailed("validation_failed", "valid trip ID is required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(errors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	var req types.PollCreate
	if !bindJSONOrError(c, &req) {
		return
	}

	resp, err := h.pollService.CreatePollWithEvent(c.Request.Context(), tripID, userID, &req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// ListPollsHandler lists polls for a trip with pagination.
func (h *PollHandler) ListPollsHandler(c *gin.Context) {
	tripID := c.Param("id")
	if tripID == "" || !isValidUUID(tripID) {
		_ = c.Error(errors.ValidationFailed("validation_failed", "valid trip ID is required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(errors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	params := getPaginationParams(c, 20, 0)

	polls, total, err := h.pollService.ListTripPolls(c.Request.Context(), tripID, userID, params.Limit, params.Offset)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": polls,
		"pagination": gin.H{
			"limit":  params.Limit,
			"offset": params.Offset,
			"total":  total,
		},
	})
}

// GetPollHandler retrieves a single poll with full results.
func (h *PollHandler) GetPollHandler(c *gin.Context) {
	tripID := c.Param("id")
	pollID := c.Param("pollID")
	if tripID == "" || !isValidUUID(tripID) || pollID == "" || !isValidUUID(pollID) {
		_ = c.Error(errors.ValidationFailed("validation_failed", "valid trip ID and poll ID are required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(errors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	resp, err := h.pollService.GetPollWithResults(c.Request.Context(), tripID, pollID, userID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// UpdatePollHandler updates a poll's question.
func (h *PollHandler) UpdatePollHandler(c *gin.Context) {
	tripID := c.Param("id")
	pollID := c.Param("pollID")
	if tripID == "" || !isValidUUID(tripID) || pollID == "" || !isValidUUID(pollID) {
		_ = c.Error(errors.ValidationFailed("validation_failed", "valid trip ID and poll ID are required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(errors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	var req types.PollUpdate
	if !bindJSONOrError(c, &req) {
		return
	}

	resp, err := h.pollService.UpdatePollWithEvent(c.Request.Context(), tripID, pollID, userID, &req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// DeletePollHandler soft-deletes a poll.
func (h *PollHandler) DeletePollHandler(c *gin.Context) {
	tripID := c.Param("id")
	pollID := c.Param("pollID")
	if tripID == "" || !isValidUUID(tripID) || pollID == "" || !isValidUUID(pollID) {
		_ = c.Error(errors.ValidationFailed("validation_failed", "valid trip ID and poll ID are required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(errors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	if err := h.pollService.DeletePollWithEvent(c.Request.Context(), tripID, pollID, userID); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Poll deleted successfully",
	})
}

// CastVoteHandler casts a vote on a poll option.
func (h *PollHandler) CastVoteHandler(c *gin.Context) {
	tripID := c.Param("id")
	pollID := c.Param("pollID")
	if tripID == "" || !isValidUUID(tripID) || pollID == "" || !isValidUUID(pollID) {
		_ = c.Error(errors.ValidationFailed("validation_failed", "valid trip ID and poll ID are required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(errors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	var req types.CastVoteRequest
	if !bindJSONOrError(c, &req) {
		return
	}

	if !isValidUUID(req.OptionID) {
		_ = c.Error(errors.ValidationFailed("validation_failed", "valid option ID is required"))
		return
	}

	if err := h.pollService.CastVoteWithEvent(c.Request.Context(), tripID, pollID, req.OptionID, userID); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Vote cast successfully",
	})
}

// RemoveVoteHandler removes a vote from a poll option.
func (h *PollHandler) RemoveVoteHandler(c *gin.Context) {
	tripID := c.Param("id")
	pollID := c.Param("pollID")
	optionID := c.Param("optionID")
	if tripID == "" || !isValidUUID(tripID) || pollID == "" || !isValidUUID(pollID) || optionID == "" || !isValidUUID(optionID) {
		_ = c.Error(errors.ValidationFailed("validation_failed", "valid trip ID, poll ID, and option ID are required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(errors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	if err := h.pollService.RemoveVoteWithEvent(c.Request.Context(), tripID, pollID, optionID, userID); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Vote removed successfully",
	})
}

// ClosePollHandler closes a poll.
func (h *PollHandler) ClosePollHandler(c *gin.Context) {
	tripID := c.Param("id")
	pollID := c.Param("pollID")
	if tripID == "" || !isValidUUID(tripID) || pollID == "" || !isValidUUID(pollID) {
		_ = c.Error(errors.ValidationFailed("validation_failed", "valid trip ID and poll ID are required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(errors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	resp, err := h.pollService.ClosePollWithEvent(c.Request.Context(), tripID, pollID, userID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
