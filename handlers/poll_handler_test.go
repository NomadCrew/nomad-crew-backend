package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ---------------------------------------------------------------------------
// Mock
// ---------------------------------------------------------------------------

type MockPollService struct {
	mock.Mock
}

func (m *MockPollService) CreatePollWithEvent(ctx context.Context, tripID, userID string, req *types.PollCreate) (*types.PollResponse, error) {
	args := m.Called(ctx, tripID, userID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.PollResponse), args.Error(1)
}

func (m *MockPollService) GetPollWithResults(ctx context.Context, tripID, pollID, userID string) (*types.PollResponse, error) {
	args := m.Called(ctx, tripID, pollID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.PollResponse), args.Error(1)
}

func (m *MockPollService) ListTripPolls(ctx context.Context, tripID, userID string, limit, offset int) ([]*types.PollResponse, int, error) {
	args := m.Called(ctx, tripID, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*types.PollResponse), args.Int(1), args.Error(2)
}

func (m *MockPollService) UpdatePollWithEvent(ctx context.Context, tripID, pollID, userID string, req *types.PollUpdate) (*types.PollResponse, error) {
	args := m.Called(ctx, tripID, pollID, userID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.PollResponse), args.Error(1)
}

func (m *MockPollService) DeletePollWithEvent(ctx context.Context, tripID, pollID, userID string) error {
	args := m.Called(ctx, tripID, pollID, userID)
	return args.Error(0)
}

func (m *MockPollService) CastVoteWithEvent(ctx context.Context, tripID, pollID, optionID, userID string) error {
	args := m.Called(ctx, tripID, pollID, optionID, userID)
	return args.Error(0)
}

func (m *MockPollService) RemoveVoteWithEvent(ctx context.Context, tripID, pollID, optionID, userID string) error {
	args := m.Called(ctx, tripID, pollID, optionID, userID)
	return args.Error(0)
}

func (m *MockPollService) ClosePollWithEvent(ctx context.Context, tripID, pollID, userID string) (*types.PollResponse, error) {
	args := m.Called(ctx, tripID, pollID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.PollResponse), args.Error(1)
}

// compile-time check
var _ PollServiceInterface = (*MockPollService)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const (
	testPollID   = "44444444-4444-4444-4444-444444444444"
	testOptionID = "55555555-5555-5555-5555-555555555555"
)

func setupPollHandler() (*PollHandler, *MockPollService) {
	svc := new(MockPollService)
	h := NewPollHandler(svc)
	return h, svc
}

func buildPollRouter(path, method string, handler gin.HandlerFunc, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		if userID != "" {
			c.Set(string(middleware.UserIDKey), userID)
		}
		c.Next()
	})
	switch method {
	case http.MethodGet:
		r.GET(path, handler)
	case http.MethodPost:
		r.POST(path, handler)
	case http.MethodPut:
		r.PUT(path, handler)
	case http.MethodDelete:
		r.DELETE(path, handler)
	}
	return r
}

func samplePollResponse(pollType types.PollType, isBlind bool) *types.PollResponse {
	now := time.Now()
	return &types.PollResponse{
		Poll: types.Poll{
			ID:        testPollID,
			TripID:    testTripID,
			Question:  "Where should we eat?",
			PollType:  pollType,
			IsBlind:   isBlind,
			Status:    types.PollStatusActive,
			CreatedBy: testUserID,
			ExpiresAt: now.Add(24 * time.Hour),
			CreatedAt: now,
			UpdatedAt: now,
		},
		Options: []types.PollOptionWithVotes{
			{
				PollOption: types.PollOption{
					ID:       testOptionID,
					PollID:   testPollID,
					Text:     "Sushi",
					Position: 0,
				},
				VoteCount: 1,
				Voters:    []types.PollVoter{{UserID: testUserID, CreatedAt: now}},
				HasVoted:  true,
			},
			{
				PollOption: types.PollOption{
					ID:       "66666666-6666-6666-6666-666666666666",
					PollID:   testPollID,
					Text:     "Pizza",
					Position: 1,
				},
				VoteCount: 0,
				Voters:    []types.PollVoter{},
				HasVoted:  false,
			},
		},
		TotalVotes:    1,
		UserVoteCount: 1,
	}
}

func sampleBlindActivePollResponse() *types.PollResponse {
	resp := samplePollResponse(types.PollTypeStandard, true)
	// Blind active: vote counts stripped, hasVoted preserved
	for i := range resp.Options {
		resp.Options[i].VoteCount = 0
		resp.Options[i].Voters = []types.PollVoter{}
	}
	resp.TotalVotes = 0
	return resp
}

func sampleClosedPollResponse(isBlind bool) *types.PollResponse {
	resp := samplePollResponse(types.PollTypeStandard, isBlind)
	resp.Status = types.PollStatusClosed
	closedBy := testUserID
	closedAt := time.Now()
	resp.ClosedBy = &closedBy
	resp.ClosedAt = &closedAt
	return resp
}

func postJSON(path string, body interface{}) (*http.Request, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func putJSON(path string, body interface{}) (*http.Request, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPut, path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// ---------------------------------------------------------------------------
// CreatePollHandler tests
// ---------------------------------------------------------------------------

func TestPollCreate_StandardPoll_Success(t *testing.T) {
	handler, svc := setupPollHandler()
	resp := samplePollResponse(types.PollTypeStandard, false)
	svc.On("CreatePollWithEvent", mock.Anything, testTripID, testUserID, mock.AnythingOfType("*types.PollCreate")).
		Return(resp, nil)

	r := buildPollRouter("/v1/trips/:id/polls", http.MethodPost, handler.CreatePollHandler, testUserID)
	req, _ := postJSON(fmt.Sprintf("/v1/trips/%s/polls", testTripID), types.PollCreate{
		Question: "Where should we eat?",
		Options:  []string{"Sushi", "Pizza"},
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, testPollID, body["id"])
	svc.AssertExpectations(t)
}

func TestPollCreate_EmojiPoll_Success(t *testing.T) {
	handler, svc := setupPollHandler()
	resp := samplePollResponse(types.PollTypeEmoji, false)
	resp.PollType = types.PollTypeEmoji
	svc.On("CreatePollWithEvent", mock.Anything, testTripID, testUserID, mock.AnythingOfType("*types.PollCreate")).
		Return(resp, nil)

	r := buildPollRouter("/v1/trips/:id/polls", http.MethodPost, handler.CreatePollHandler, testUserID)
	req, _ := postJSON(fmt.Sprintf("/v1/trips/%s/polls", testTripID), types.PollCreate{
		Question: "How are you feeling?",
		Options:  []string{"😀", "😢"},
		PollType: types.PollTypeEmoji,
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestPollCreate_BinaryPoll_Success(t *testing.T) {
	handler, svc := setupPollHandler()
	resp := samplePollResponse(types.PollTypeBinary, false)
	resp.PollType = types.PollTypeBinary
	svc.On("CreatePollWithEvent", mock.Anything, testTripID, testUserID, mock.AnythingOfType("*types.PollCreate")).
		Return(resp, nil)

	r := buildPollRouter("/v1/trips/:id/polls", http.MethodPost, handler.CreatePollHandler, testUserID)
	req, _ := postJSON(fmt.Sprintf("/v1/trips/%s/polls", testTripID), types.PollCreate{
		Question: "Beach or mountains?",
		Options:  []string{"Beach", "Mountains"},
		PollType: types.PollTypeBinary,
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestPollCreate_BinaryPoll_ThreeOptions_Rejected(t *testing.T) {
	handler, svc := setupPollHandler()
	svc.On("CreatePollWithEvent", mock.Anything, testTripID, testUserID, mock.AnythingOfType("*types.PollCreate")).
		Return(nil, apperrors.ValidationFailed("Invalid poll data", "binary polls must have exactly 2 options"))

	r := buildPollRouter("/v1/trips/:id/polls", http.MethodPost, handler.CreatePollHandler, testUserID)
	req, _ := postJSON(fmt.Sprintf("/v1/trips/%s/polls", testTripID), types.PollCreate{
		Question: "Pick one",
		Options:  []string{"A", "B", "C"},
		PollType: types.PollTypeBinary,
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestPollCreate_BlindPoll_Success(t *testing.T) {
	handler, svc := setupPollHandler()
	resp := sampleBlindActivePollResponse()
	svc.On("CreatePollWithEvent", mock.Anything, testTripID, testUserID, mock.AnythingOfType("*types.PollCreate")).
		Return(resp, nil)

	r := buildPollRouter("/v1/trips/:id/polls", http.MethodPost, handler.CreatePollHandler, testUserID)
	req, _ := postJSON(fmt.Sprintf("/v1/trips/%s/polls", testTripID), types.PollCreate{
		Question: "Secret vote",
		Options:  []string{"Yes", "No"},
		IsBlind:  true,
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestPollCreate_InvalidPollType_Rejected(t *testing.T) {
	handler, svc := setupPollHandler()
	svc.On("CreatePollWithEvent", mock.Anything, testTripID, testUserID, mock.AnythingOfType("*types.PollCreate")).
		Return(nil, apperrors.ValidationFailed("Invalid poll data", "invalid poll type: bogus"))

	r := buildPollRouter("/v1/trips/:id/polls", http.MethodPost, handler.CreatePollHandler, testUserID)
	req, _ := postJSON(fmt.Sprintf("/v1/trips/%s/polls", testTripID), types.PollCreate{
		Question: "Test",
		Options:  []string{"A", "B"},
		PollType: "bogus",
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestPollCreate_MissingQuestion_BindError(t *testing.T) {
	handler, _ := setupPollHandler()
	r := buildPollRouter("/v1/trips/:id/polls", http.MethodPost, handler.CreatePollHandler, testUserID)
	// Send JSON with empty question — Gin binding may reject, or service may reject
	req, _ := postJSON(fmt.Sprintf("/v1/trips/%s/polls", testTripID), map[string]interface{}{
		"options": []string{"A", "B"},
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPollCreate_Unauthenticated(t *testing.T) {
	handler, _ := setupPollHandler()
	r := buildPollRouter("/v1/trips/:id/polls", http.MethodPost, handler.CreatePollHandler, "")
	req, _ := postJSON(fmt.Sprintf("/v1/trips/%s/polls", testTripID), types.PollCreate{
		Question: "Test",
		Options:  []string{"A", "B"},
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPollCreate_InvalidTripUUID(t *testing.T) {
	handler, _ := setupPollHandler()
	r := buildPollRouter("/v1/trips/:id/polls", http.MethodPost, handler.CreatePollHandler, testUserID)
	req, _ := postJSON("/v1/trips/not-a-uuid/polls", types.PollCreate{
		Question: "Test",
		Options:  []string{"A", "B"},
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// CastVoteHandler tests
// ---------------------------------------------------------------------------

func TestPollVote_ActivePoll_Success(t *testing.T) {
	handler, svc := setupPollHandler()
	svc.On("CastVoteWithEvent", mock.Anything, testTripID, testPollID, testOptionID, testUserID).
		Return(nil)

	r := buildPollRouter("/v1/trips/:id/polls/:pollID/votes", http.MethodPost, handler.CastVoteHandler, testUserID)
	req, _ := postJSON(fmt.Sprintf("/v1/trips/%s/polls/%s/votes", testTripID, testPollID), types.CastVoteRequest{
		OptionID: testOptionID,
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestPollVote_BlindPoll_Success(t *testing.T) {
	handler, svc := setupPollHandler()
	// Service accepts the vote regardless of blind — handler doesn't know/care
	svc.On("CastVoteWithEvent", mock.Anything, testTripID, testPollID, testOptionID, testUserID).
		Return(nil)

	r := buildPollRouter("/v1/trips/:id/polls/:pollID/votes", http.MethodPost, handler.CastVoteHandler, testUserID)
	req, _ := postJSON(fmt.Sprintf("/v1/trips/%s/polls/%s/votes", testTripID, testPollID), types.CastVoteRequest{
		OptionID: testOptionID,
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestPollVote_ExpiredPoll_Rejected(t *testing.T) {
	handler, svc := setupPollHandler()
	svc.On("CastVoteWithEvent", mock.Anything, testTripID, testPollID, testOptionID, testUserID).
		Return(apperrors.ValidationFailed("poll_expired", "cannot vote on an expired poll"))

	r := buildPollRouter("/v1/trips/:id/polls/:pollID/votes", http.MethodPost, handler.CastVoteHandler, testUserID)
	req, _ := postJSON(fmt.Sprintf("/v1/trips/%s/polls/%s/votes", testTripID, testPollID), types.CastVoteRequest{
		OptionID: testOptionID,
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestPollVote_ClosedPoll_Rejected(t *testing.T) {
	handler, svc := setupPollHandler()
	svc.On("CastVoteWithEvent", mock.Anything, testTripID, testPollID, testOptionID, testUserID).
		Return(apperrors.ValidationFailed("poll_closed", "cannot vote on a closed poll"))

	r := buildPollRouter("/v1/trips/:id/polls/:pollID/votes", http.MethodPost, handler.CastVoteHandler, testUserID)
	req, _ := postJSON(fmt.Sprintf("/v1/trips/%s/polls/%s/votes", testTripID, testPollID), types.CastVoteRequest{
		OptionID: testOptionID,
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestPollVote_InvalidOptionUUID(t *testing.T) {
	handler, _ := setupPollHandler()
	r := buildPollRouter("/v1/trips/:id/polls/:pollID/votes", http.MethodPost, handler.CastVoteHandler, testUserID)
	req, _ := postJSON(fmt.Sprintf("/v1/trips/%s/polls/%s/votes", testTripID, testPollID), types.CastVoteRequest{
		OptionID: "not-a-uuid",
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPollVote_Unauthenticated(t *testing.T) {
	handler, _ := setupPollHandler()
	r := buildPollRouter("/v1/trips/:id/polls/:pollID/votes", http.MethodPost, handler.CastVoteHandler, "")
	req, _ := postJSON(fmt.Sprintf("/v1/trips/%s/polls/%s/votes", testTripID, testPollID), types.CastVoteRequest{
		OptionID: testOptionID,
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ---------------------------------------------------------------------------
// RemoveVoteHandler tests
// ---------------------------------------------------------------------------

func TestPollRemoveVote_Success(t *testing.T) {
	handler, svc := setupPollHandler()
	svc.On("RemoveVoteWithEvent", mock.Anything, testTripID, testPollID, testOptionID, testUserID).
		Return(nil)

	r := buildPollRouter("/v1/trips/:id/polls/:pollID/votes/:optionID", http.MethodDelete, handler.RemoveVoteHandler, testUserID)
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/trips/%s/polls/%s/votes/%s", testTripID, testPollID, testOptionID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestPollRemoveVote_ClosedPoll_Rejected(t *testing.T) {
	handler, svc := setupPollHandler()
	svc.On("RemoveVoteWithEvent", mock.Anything, testTripID, testPollID, testOptionID, testUserID).
		Return(apperrors.ValidationFailed("poll_closed", "cannot modify votes on a closed poll"))

	r := buildPollRouter("/v1/trips/:id/polls/:pollID/votes/:optionID", http.MethodDelete, handler.RemoveVoteHandler, testUserID)
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/trips/%s/polls/%s/votes/%s", testTripID, testPollID, testOptionID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// GetPollHandler tests
// ---------------------------------------------------------------------------

func TestPollGet_StandardPoll_FullResults(t *testing.T) {
	handler, svc := setupPollHandler()
	resp := samplePollResponse(types.PollTypeStandard, false)
	svc.On("GetPollWithResults", mock.Anything, testTripID, testPollID, testUserID).
		Return(resp, nil)

	r := buildPollRouter("/v1/trips/:id/polls/:pollID", http.MethodGet, handler.GetPollHandler, testUserID)
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/trips/%s/polls/%s", testTripID, testPollID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, testPollID, body["id"])
	assert.Equal(t, float64(1), body["totalVotes"])
	svc.AssertExpectations(t)
}

func TestPollGet_BlindActivePoll_StrippedResults(t *testing.T) {
	handler, svc := setupPollHandler()
	resp := sampleBlindActivePollResponse()
	svc.On("GetPollWithResults", mock.Anything, testTripID, testPollID, testUserID).
		Return(resp, nil)

	r := buildPollRouter("/v1/trips/:id/polls/:pollID", http.MethodGet, handler.GetPollHandler, testUserID)
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/trips/%s/polls/%s", testTripID, testPollID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(0), body["totalVotes"])
	// Options should have voteCount=0 and empty voters
	opts := body["options"].([]interface{})
	for _, opt := range opts {
		optMap := opt.(map[string]interface{})
		assert.Equal(t, float64(0), optMap["voteCount"])
		assert.Empty(t, optMap["voters"])
	}
	svc.AssertExpectations(t)
}

func TestPollGet_BlindClosedPoll_FullResults(t *testing.T) {
	handler, svc := setupPollHandler()
	resp := sampleClosedPollResponse(true)
	svc.On("GetPollWithResults", mock.Anything, testTripID, testPollID, testUserID).
		Return(resp, nil)

	r := buildPollRouter("/v1/trips/:id/polls/:pollID", http.MethodGet, handler.GetPollHandler, testUserID)
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/trips/%s/polls/%s", testTripID, testPollID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	// Closed blind poll returns full results
	assert.Equal(t, float64(1), body["totalVotes"])
	svc.AssertExpectations(t)
}

func TestPollGet_NotFound(t *testing.T) {
	handler, svc := setupPollHandler()
	svc.On("GetPollWithResults", mock.Anything, testTripID, testPollID, testUserID).
		Return(nil, apperrors.NotFound("poll", testPollID))

	r := buildPollRouter("/v1/trips/:id/polls/:pollID", http.MethodGet, handler.GetPollHandler, testUserID)
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/trips/%s/polls/%s", testTripID, testPollID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

func TestPollGet_Unauthenticated(t *testing.T) {
	handler, _ := setupPollHandler()
	r := buildPollRouter("/v1/trips/:id/polls/:pollID", http.MethodGet, handler.GetPollHandler, "")
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/trips/%s/polls/%s", testTripID, testPollID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ---------------------------------------------------------------------------
// ClosePollHandler tests
// ---------------------------------------------------------------------------

func TestPollClose_ExpiredPoll_Success(t *testing.T) {
	handler, svc := setupPollHandler()
	resp := sampleClosedPollResponse(false)
	svc.On("ClosePollWithEvent", mock.Anything, testTripID, testPollID, testUserID).
		Return(resp, nil)

	r := buildPollRouter("/v1/trips/:id/polls/:pollID/close", http.MethodPost, handler.ClosePollHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/trips/%s/polls/%s/close", testTripID, testPollID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, string(types.PollStatusClosed), body["status"])
	svc.AssertExpectations(t)
}

func TestPollClose_AllMembersVoted_Success(t *testing.T) {
	handler, svc := setupPollHandler()
	resp := sampleClosedPollResponse(false)
	svc.On("ClosePollWithEvent", mock.Anything, testTripID, testPollID, testUserID).
		Return(resp, nil)

	r := buildPollRouter("/v1/trips/:id/polls/:pollID/close", http.MethodPost, handler.ClosePollHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/trips/%s/polls/%s/close", testTripID, testPollID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestPollClose_BlindPoll_ReturnsResults(t *testing.T) {
	handler, svc := setupPollHandler()
	resp := sampleClosedPollResponse(true)
	svc.On("ClosePollWithEvent", mock.Anything, testTripID, testPollID, testUserID).
		Return(resp, nil)

	r := buildPollRouter("/v1/trips/:id/polls/:pollID/close", http.MethodPost, handler.ClosePollHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/trips/%s/polls/%s/close", testTripID, testPollID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, true, body["isBlind"])
	assert.Equal(t, string(types.PollStatusClosed), body["status"])
	svc.AssertExpectations(t)
}

func TestPollClose_Unauthorized(t *testing.T) {
	handler, svc := setupPollHandler()
	svc.On("ClosePollWithEvent", mock.Anything, testTripID, testPollID, testUserID).
		Return(nil, apperrors.Forbidden("unauthorized", "you don't have permission to close this poll"))

	r := buildPollRouter("/v1/trips/:id/polls/:pollID/close", http.MethodPost, handler.ClosePollHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/trips/%s/polls/%s/close", testTripID, testPollID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertExpectations(t)
}

func TestPollClose_Unauthenticated(t *testing.T) {
	handler, _ := setupPollHandler()
	r := buildPollRouter("/v1/trips/:id/polls/:pollID/close", http.MethodPost, handler.ClosePollHandler, "")
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/trips/%s/polls/%s/close", testTripID, testPollID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPollClose_NotYetCloseable(t *testing.T) {
	handler, svc := setupPollHandler()
	svc.On("ClosePollWithEvent", mock.Anything, testTripID, testPollID, testUserID).
		Return(nil, apperrors.ValidationFailed("poll_close_restricted", "poll cannot be closed until all members have voted or the poll expires"))

	r := buildPollRouter("/v1/trips/:id/polls/:pollID/close", http.MethodPost, handler.ClosePollHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/trips/%s/polls/%s/close", testTripID, testPollID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// ListPollsHandler tests
// ---------------------------------------------------------------------------

func TestPollList_Success(t *testing.T) {
	handler, svc := setupPollHandler()
	polls := []*types.PollResponse{samplePollResponse(types.PollTypeStandard, false)}
	svc.On("ListTripPolls", mock.Anything, testTripID, testUserID, 20, 0).
		Return(polls, 1, nil)

	r := buildPollRouter("/v1/trips/:id/polls", http.MethodGet, handler.ListPollsHandler, testUserID)
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/trips/%s/polls", testTripID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(1), body["pagination"].(map[string]interface{})["total"])
	svc.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// DeletePollHandler tests
// ---------------------------------------------------------------------------

func TestPollDelete_Success(t *testing.T) {
	handler, svc := setupPollHandler()
	svc.On("DeletePollWithEvent", mock.Anything, testTripID, testPollID, testUserID).
		Return(nil)

	r := buildPollRouter("/v1/trips/:id/polls/:pollID", http.MethodDelete, handler.DeletePollHandler, testUserID)
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/trips/%s/polls/%s", testTripID, testPollID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "Poll deleted successfully", body["message"])
	svc.AssertExpectations(t)
}

func TestPollDelete_Forbidden(t *testing.T) {
	handler, svc := setupPollHandler()
	svc.On("DeletePollWithEvent", mock.Anything, testTripID, testPollID, testUserID).
		Return(apperrors.Forbidden("unauthorized", "you don't have permission to delete this poll"))

	r := buildPollRouter("/v1/trips/:id/polls/:pollID", http.MethodDelete, handler.DeletePollHandler, testUserID)
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/trips/%s/polls/%s", testTripID, testPollID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertExpectations(t)
}
