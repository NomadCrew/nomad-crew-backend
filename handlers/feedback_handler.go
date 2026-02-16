package handlers

import (
	"net/http"
	"strings"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
)

// FeedbackHandler handles feedback submission endpoints.
type FeedbackHandler struct {
	feedbackStore store.FeedbackStore
}

// NewFeedbackHandler creates a new FeedbackHandler.
func NewFeedbackHandler(feedbackStore store.FeedbackStore) *FeedbackHandler {
	return &FeedbackHandler{feedbackStore: feedbackStore}
}

// SubmitFeedback godoc
// @Summary      Submit feedback
// @Description  Submit feedback from the landing page or app
// @Tags         feedback
// @Accept       json
// @Produce      json
// @Param        body  body      types.FeedbackCreate  true  "Feedback payload"
// @Success      201   {object}  types.StatusResponse
// @Failure      400   {object}  types.ErrorResponse
// @Failure      429   {object}  types.ErrorResponse
// @Failure      500   {object}  types.ErrorResponse
// @Router       /feedback [post]
func (h *FeedbackHandler) SubmitFeedback(c *gin.Context) {
	var req types.FeedbackCreate
	if !bindJSONOrError(c, &req) {
		return
	}

	// Trim whitespace and re-validate
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)
	req.Message = strings.TrimSpace(req.Message)

	if req.Name == "" {
		_ = c.Error(errors.ValidationFailed("validation_failed", "name must not be blank"))
		return
	}
	if len(req.Message) < 10 {
		_ = c.Error(errors.ValidationFailed("validation_failed", "message must be at least 10 characters after trimming"))
		return
	}

	// Default source to "landing" if not provided (backward-compatible with landing page)
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "landing"
	}

	allowedSources := map[string]bool{
		"landing":      true,
		"app_bug":      true,
		"app_feedback": true,
	}
	if !allowedSources[source] {
		_ = c.Error(errors.ValidationFailed("validation_failed", "source must be one of: landing, app_bug, app_feedback"))
		return
	}

	fb := &types.Feedback{
		Name:    req.Name,
		Email:   req.Email,
		Message: req.Message,
		Source:  source,
	}

	_, err := h.feedbackStore.CreateFeedback(c.Request.Context(), fb)
	if err != nil {
		_ = c.Error(errors.NewDatabaseError(err))
		return
	}

	c.JSON(http.StatusCreated, types.StatusResponse{Status: "Feedback submitted successfully"})
}
