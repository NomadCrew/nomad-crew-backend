package handlers

import (
	"net/http"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PollHandler struct {
	pollModel *models.PollModel
}

func NewPollHandler(model *models.PollModel) *PollHandler {
	return &PollHandler{
		pollModel: model,
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

	resp, err := h.pollModel.CreatePollWithEvent(c.Request.Context(), tripID, userID, &req)
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

	polls, total, err := h.pollModel.ListTripPolls(c.Request.Context(), tripID, userID, params.Limit, params.Offset)
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

	resp, err := h.pollModel.GetPollWithResults(c.Request.Context(), tripID, pollID, userID)
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

	resp, err := h.pollModel.UpdatePollWithEvent(c.Request.Context(), tripID, pollID, userID, &req)
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

	if err := h.pollModel.DeletePollWithEvent(c.Request.Context(), tripID, pollID, userID); err != nil {
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

	if err := h.pollModel.CastVoteWithEvent(c.Request.Context(), tripID, pollID, req.OptionID, userID); err != nil {
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

	if err := h.pollModel.RemoveVoteWithEvent(c.Request.Context(), tripID, pollID, optionID, userID); err != nil {
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

	resp, err := h.pollModel.ClosePollWithEvent(c.Request.Context(), tripID, pollID, userID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
