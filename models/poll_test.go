package models_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

type MockPollStore struct {
	mock.Mock
}

var _ store.PollStore = (*MockPollStore)(nil)

func (m *MockPollStore) CreatePoll(ctx context.Context, poll *types.Poll) (string, error) {
	args := m.Called(ctx, poll)
	return args.String(0), args.Error(1)
}

func (m *MockPollStore) CreatePollWithOptions(ctx context.Context, poll *types.Poll, options []*types.PollOption) (string, error) {
	args := m.Called(ctx, poll, options)
	return args.String(0), args.Error(1)
}

func (m *MockPollStore) GetPoll(ctx context.Context, id, tripID string) (*types.Poll, error) {
	args := m.Called(ctx, id, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Poll), args.Error(1)
}

func (m *MockPollStore) ListPolls(ctx context.Context, tripID string, limit, offset int) ([]*types.Poll, int, error) {
	args := m.Called(ctx, tripID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*types.Poll), args.Int(1), args.Error(2)
}

func (m *MockPollStore) UpdatePollQuestion(ctx context.Context, id, tripID, question string) (*types.Poll, error) {
	args := m.Called(ctx, id, tripID, question)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Poll), args.Error(1)
}

func (m *MockPollStore) ClosePoll(ctx context.Context, id, tripID, closedBy string) (*types.Poll, error) {
	args := m.Called(ctx, id, tripID, closedBy)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Poll), args.Error(1)
}

func (m *MockPollStore) SoftDeletePoll(ctx context.Context, id, tripID string) error {
	args := m.Called(ctx, id, tripID)
	return args.Error(0)
}

func (m *MockPollStore) CreatePollOption(ctx context.Context, option *types.PollOption) (string, error) {
	args := m.Called(ctx, option)
	return args.String(0), args.Error(1)
}

func (m *MockPollStore) ListPollOptions(ctx context.Context, pollID string) ([]*types.PollOption, error) {
	args := m.Called(ctx, pollID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.PollOption), args.Error(1)
}

func (m *MockPollStore) CastVote(ctx context.Context, pollID, optionID, userID string) error {
	args := m.Called(ctx, pollID, optionID, userID)
	return args.Error(0)
}

func (m *MockPollStore) SwapVote(ctx context.Context, pollID, optionID, userID string) error {
	args := m.Called(ctx, pollID, optionID, userID)
	return args.Error(0)
}

func (m *MockPollStore) RemoveVote(ctx context.Context, pollID, optionID, userID string) error {
	args := m.Called(ctx, pollID, optionID, userID)
	return args.Error(0)
}

func (m *MockPollStore) RemoveAllUserVotesForPoll(ctx context.Context, pollID, userID string) error {
	args := m.Called(ctx, pollID, userID)
	return args.Error(0)
}

func (m *MockPollStore) GetUserVotesForPoll(ctx context.Context, pollID, userID string) ([]*types.PollVote, error) {
	args := m.Called(ctx, pollID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.PollVote), args.Error(1)
}

func (m *MockPollStore) GetVoteCountsByPoll(ctx context.Context, pollID string) (map[string]int, error) {
	args := m.Called(ctx, pollID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockPollStore) ListVotesByPoll(ctx context.Context, pollID string) ([]*types.PollVote, error) {
	args := m.Called(ctx, pollID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.PollVote), args.Error(1)
}

func (m *MockPollStore) CountUniqueVotersByPoll(ctx context.Context, pollID string) (int, error) {
	args := m.Called(ctx, pollID)
	return args.Int(0), args.Error(1)
}

func (m *MockPollStore) BeginTx(ctx context.Context) (types.DatabaseTransaction, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(types.DatabaseTransaction), args.Error(1)
}

// --- Helpers ---

func validStandardPollCreate() *types.PollCreate {
	return &types.PollCreate{
		Question: "Where should we eat?",
		Options:  []string{"Pizza", "Sushi", "Tacos"},
		PollType: types.PollTypeStandard,
	}
}

// setupPollModel creates a PollModel with fresh mocks for each test.
func setupPollModel() (*models.PollModel, *MockPollStore, *MockTripModel, *MockEventPublisher) {
	pollStore := new(MockPollStore)
	tripModel := new(MockTripModel)
	eventPub := new(MockEventPublisher)
	pm := models.NewPollModel(pollStore, tripModel, eventPub)
	return pm, pollStore, tripModel, eventPub
}

// stubBuildPollResponse sets up the mocks needed by buildPollResponse so
// that CreatePollWithEvent and ClosePollWithEvent can reach the event-publishing
// code path without failing on the response-building step.
func stubBuildPollResponse(pollStore *MockPollStore, pollID string, options []*types.PollOption) {
	pollStore.On("ListPollOptions", mock.Anything, pollID).Return(options, nil).Maybe()
	pollStore.On("GetVoteCountsByPoll", mock.Anything, pollID).Return(map[string]int{}, nil).Maybe()
	pollStore.On("ListVotesByPoll", mock.Anything, pollID).Return([]*types.PollVote{}, nil).Maybe()
	pollStore.On("GetUserVotesForPoll", mock.Anything, pollID, mock.Anything).Return([]*types.PollVote{}, nil).Maybe()
}

// =============================================================================
// VALIDATION TESTS
// =============================================================================

func TestPollValidation_ValidStandard(t *testing.T) {
	pm, pollStore, tripModel, eventPub := setupPollModel()
	ctx := context.Background()

	req := validStandardPollCreate()
	tripModel.On("GetUserRole", ctx, "trip-1", "user-1").Return(types.MemberRoleMember, nil)
	pollStore.On("CreatePollWithOptions", ctx, mock.Anything, mock.Anything).Return("poll-1", nil)
	pollStore.On("GetPoll", ctx, "poll-1", "trip-1").Return(&types.Poll{
		ID: "poll-1", TripID: "trip-1", Status: types.PollStatusActive, PollType: types.PollTypeStandard,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}, nil)
	stubBuildPollResponse(pollStore, "poll-1", []*types.PollOption{})
	eventPub.On("Publish", mock.Anything, "trip-1", mock.Anything).Return(nil)

	resp, err := pm.CreatePollWithEvent(ctx, "trip-1", "user-1", req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	pollStore.AssertExpectations(t)
}

func TestPollValidation_ValidEmoji(t *testing.T) {
	pm, pollStore, tripModel, eventPub := setupPollModel()
	ctx := context.Background()

	req := &types.PollCreate{
		Question: "Rate the vibe",
		Options:  []string{"fire", "fire", "thumbsup"},
		PollType: types.PollTypeEmoji,
	}
	tripModel.On("GetUserRole", ctx, "trip-1", "user-1").Return(types.MemberRoleMember, nil)
	pollStore.On("CreatePollWithOptions", ctx, mock.Anything, mock.Anything).Return("poll-1", nil)
	pollStore.On("GetPoll", ctx, "poll-1", "trip-1").Return(&types.Poll{
		ID: "poll-1", TripID: "trip-1", Status: types.PollStatusActive, PollType: types.PollTypeEmoji,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}, nil)
	stubBuildPollResponse(pollStore, "poll-1", []*types.PollOption{})
	eventPub.On("Publish", mock.Anything, "trip-1", mock.Anything).Return(nil)

	resp, err := pm.CreatePollWithEvent(ctx, "trip-1", "user-1", req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestPollValidation_ValidBinary(t *testing.T) {
	pm, pollStore, tripModel, eventPub := setupPollModel()
	ctx := context.Background()

	req := &types.PollCreate{
		Question: "Beach or mountain?",
		Options:  []string{"Beach", "Mountain"},
		PollType: types.PollTypeBinary,
	}
	tripModel.On("GetUserRole", ctx, "trip-1", "user-1").Return(types.MemberRoleMember, nil)
	pollStore.On("CreatePollWithOptions", ctx, mock.Anything, mock.Anything).Return("poll-1", nil)
	pollStore.On("GetPoll", ctx, "poll-1", "trip-1").Return(&types.Poll{
		ID: "poll-1", TripID: "trip-1", Status: types.PollStatusActive, PollType: types.PollTypeBinary,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}, nil)
	stubBuildPollResponse(pollStore, "poll-1", []*types.PollOption{})
	eventPub.On("Publish", mock.Anything, "trip-1", mock.Anything).Return(nil)

	resp, err := pm.CreatePollWithEvent(ctx, "trip-1", "user-1", req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestPollValidation_BinaryWrongOptionCount(t *testing.T) {
	pm, _, _, _ := setupPollModel()
	ctx := context.Background()

	req := &types.PollCreate{
		Question: "Pick one?",
		Options:  []string{"A", "B", "C"},
		PollType: types.PollTypeBinary,
	}
	_, err := pm.CreatePollWithEvent(ctx, "trip-1", "user-1", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "binary polls must have exactly 2 options")
}

func TestPollValidation_InvalidType(t *testing.T) {
	pm, _, _, _ := setupPollModel()
	ctx := context.Background()

	req := &types.PollCreate{
		Question: "Test?",
		Options:  []string{"A", "B"},
		PollType: "nonexistent_type",
	}
	_, err := pm.CreatePollWithEvent(ctx, "trip-1", "user-1", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid poll type")
}

func TestPollValidation_EmptyQuestion(t *testing.T) {
	pm, _, _, _ := setupPollModel()
	ctx := context.Background()

	req := &types.PollCreate{
		Question: "",
		Options:  []string{"A", "B"},
		PollType: types.PollTypeStandard,
	}
	_, err := pm.CreatePollWithEvent(ctx, "trip-1", "user-1", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "question is required")
}

func TestPollValidation_TooManyOptions(t *testing.T) {
	pm, _, _, _ := setupPollModel()
	ctx := context.Background()

	opts := make([]string, 21)
	for i := range opts {
		opts[i] = strings.Repeat("x", i+1)
	}
	req := &types.PollCreate{
		Question: "Big poll?",
		Options:  opts,
		PollType: types.PollTypeStandard,
	}
	_, err := pm.CreatePollWithEvent(ctx, "trip-1", "user-1", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum 20 options")
}

func TestPollValidation_DuplicateOptionsAllowedForEmoji(t *testing.T) {
	pm, pollStore, tripModel, eventPub := setupPollModel()
	ctx := context.Background()

	req := &types.PollCreate{
		Question: "Emoji duplicates",
		Options:  []string{"smile", "smile"},
		PollType: types.PollTypeEmoji,
	}
	tripModel.On("GetUserRole", ctx, "trip-1", "user-1").Return(types.MemberRoleMember, nil)
	pollStore.On("CreatePollWithOptions", ctx, mock.Anything, mock.Anything).Return("poll-1", nil)
	pollStore.On("GetPoll", ctx, "poll-1", "trip-1").Return(&types.Poll{
		ID: "poll-1", TripID: "trip-1", Status: types.PollStatusActive, PollType: types.PollTypeEmoji,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}, nil)
	stubBuildPollResponse(pollStore, "poll-1", []*types.PollOption{})
	eventPub.On("Publish", mock.Anything, "trip-1", mock.Anything).Return(nil)

	resp, err := pm.CreatePollWithEvent(ctx, "trip-1", "user-1", req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestPollValidation_DefaultsToStandard(t *testing.T) {
	pm, pollStore, tripModel, eventPub := setupPollModel()
	ctx := context.Background()

	req := &types.PollCreate{
		Question: "No type set",
		Options:  []string{"A", "B"},
		// PollType deliberately empty
	}
	tripModel.On("GetUserRole", ctx, "trip-1", "user-1").Return(types.MemberRoleMember, nil)
	pollStore.On("CreatePollWithOptions", ctx, mock.MatchedBy(func(p *types.Poll) bool {
		return p.PollType == types.PollTypeStandard
	}), mock.Anything).Return("poll-1", nil)
	pollStore.On("GetPoll", ctx, "poll-1", "trip-1").Return(&types.Poll{
		ID: "poll-1", TripID: "trip-1", Status: types.PollStatusActive, PollType: types.PollTypeStandard,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}, nil)
	stubBuildPollResponse(pollStore, "poll-1", []*types.PollOption{})
	eventPub.On("Publish", mock.Anything, "trip-1", mock.Anything).Return(nil)

	resp, err := pm.CreatePollWithEvent(ctx, "trip-1", "user-1", req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	// Verify the PollType was defaulted
	assert.Equal(t, types.PollTypeStandard, req.PollType)
}

// =============================================================================
// BLIND POLL RESPONSE TESTS
// =============================================================================

func TestPollBuildResponse_BlindActiveStripsVotesAndVoters(t *testing.T) {
	pm, pollStore, tripModel, eventPub := setupPollModel()
	ctx := context.Background()

	// Create a blind active poll via GetPollWithResults which calls buildPollResponse
	tripModel.On("GetUserRole", ctx, "trip-1", "user-1").Return(types.MemberRoleMember, nil)

	blindPoll := &types.Poll{
		ID: "poll-1", TripID: "trip-1", Question: "Secret vote",
		PollType: types.PollTypeStandard, IsBlind: true,
		Status: types.PollStatusActive, ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	pollStore.On("GetPoll", ctx, "poll-1", "trip-1").Return(blindPoll, nil)

	options := []*types.PollOption{
		{ID: "opt-1", PollID: "poll-1", Text: "A"},
		{ID: "opt-2", PollID: "poll-1", Text: "B"},
	}
	pollStore.On("ListPollOptions", ctx, "poll-1").Return(options, nil)
	pollStore.On("GetVoteCountsByPoll", ctx, "poll-1").Return(map[string]int{"opt-1": 3, "opt-2": 2}, nil)
	pollStore.On("ListVotesByPoll", ctx, "poll-1").Return([]*types.PollVote{
		{PollID: "poll-1", OptionID: "opt-1", UserID: "user-1"},
		{PollID: "poll-1", OptionID: "opt-1", UserID: "user-2"},
		{PollID: "poll-1", OptionID: "opt-1", UserID: "user-3"},
		{PollID: "poll-1", OptionID: "opt-2", UserID: "user-4"},
		{PollID: "poll-1", OptionID: "opt-2", UserID: "user-5"},
	}, nil)
	pollStore.On("GetUserVotesForPoll", ctx, "poll-1", "user-1").Return([]*types.PollVote{
		{PollID: "poll-1", OptionID: "opt-1", UserID: "user-1"},
	}, nil)

	_ = eventPub // not used for GetPollWithResults

	resp, err := pm.GetPollWithResults(ctx, "trip-1", "poll-1", "user-1")
	require.NoError(t, err)

	// Blind + active: VoteCount must be 0, Voters must be empty
	for _, opt := range resp.Options {
		assert.Equal(t, 0, opt.VoteCount, "blind active poll should strip VoteCount")
		assert.Empty(t, opt.Voters, "blind active poll should strip Voters")
	}
	assert.Equal(t, 0, resp.TotalVotes, "blind active poll should strip TotalVotes")
}

func TestPollBuildResponse_BlindActivePreservesHasVoted(t *testing.T) {
	pm, pollStore, tripModel, _ := setupPollModel()
	ctx := context.Background()

	tripModel.On("GetUserRole", ctx, "trip-1", "user-1").Return(types.MemberRoleMember, nil)

	blindPoll := &types.Poll{
		ID: "poll-1", TripID: "trip-1", Question: "Secret vote",
		PollType: types.PollTypeStandard, IsBlind: true,
		Status: types.PollStatusActive, ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	pollStore.On("GetPoll", ctx, "poll-1", "trip-1").Return(blindPoll, nil)

	options := []*types.PollOption{
		{ID: "opt-1", PollID: "poll-1", Text: "A"},
		{ID: "opt-2", PollID: "poll-1", Text: "B"},
	}
	pollStore.On("ListPollOptions", ctx, "poll-1").Return(options, nil)
	pollStore.On("GetVoteCountsByPoll", ctx, "poll-1").Return(map[string]int{"opt-1": 1}, nil)
	pollStore.On("ListVotesByPoll", ctx, "poll-1").Return([]*types.PollVote{
		{PollID: "poll-1", OptionID: "opt-1", UserID: "user-1"},
	}, nil)
	pollStore.On("GetUserVotesForPoll", ctx, "poll-1", "user-1").Return([]*types.PollVote{
		{PollID: "poll-1", OptionID: "opt-1", UserID: "user-1"},
	}, nil)

	resp, err := pm.GetPollWithResults(ctx, "trip-1", "poll-1", "user-1")
	require.NoError(t, err)

	// HasVoted must still be true for the option the user voted on
	var hasVotedOpt1 bool
	for _, opt := range resp.Options {
		if opt.ID == "opt-1" {
			hasVotedOpt1 = opt.HasVoted
		}
	}
	assert.True(t, hasVotedOpt1, "blind active poll should preserve HasVoted")
}

func TestPollBuildResponse_BlindClosedReturnsFullData(t *testing.T) {
	pm, pollStore, tripModel, _ := setupPollModel()
	ctx := context.Background()

	tripModel.On("GetUserRole", ctx, "trip-1", "user-1").Return(types.MemberRoleMember, nil)

	closedBy := "user-1"
	closedAt := time.Now()
	blindClosedPoll := &types.Poll{
		ID: "poll-1", TripID: "trip-1", Question: "Secret vote",
		PollType: types.PollTypeStandard, IsBlind: true,
		Status: types.PollStatusClosed, ClosedBy: &closedBy, ClosedAt: &closedAt,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	pollStore.On("GetPoll", ctx, "poll-1", "trip-1").Return(blindClosedPoll, nil)

	options := []*types.PollOption{
		{ID: "opt-1", PollID: "poll-1", Text: "A"},
		{ID: "opt-2", PollID: "poll-1", Text: "B"},
	}
	pollStore.On("ListPollOptions", ctx, "poll-1").Return(options, nil)
	pollStore.On("GetVoteCountsByPoll", ctx, "poll-1").Return(map[string]int{"opt-1": 3, "opt-2": 2}, nil)
	pollStore.On("ListVotesByPoll", ctx, "poll-1").Return([]*types.PollVote{
		{PollID: "poll-1", OptionID: "opt-1", UserID: "user-1"},
		{PollID: "poll-1", OptionID: "opt-1", UserID: "user-2"},
		{PollID: "poll-1", OptionID: "opt-1", UserID: "user-3"},
		{PollID: "poll-1", OptionID: "opt-2", UserID: "user-4"},
		{PollID: "poll-1", OptionID: "opt-2", UserID: "user-5"},
	}, nil)
	pollStore.On("GetUserVotesForPoll", ctx, "poll-1", "user-1").Return([]*types.PollVote{
		{PollID: "poll-1", OptionID: "opt-1", UserID: "user-1"},
	}, nil)

	resp, err := pm.GetPollWithResults(ctx, "trip-1", "poll-1", "user-1")
	require.NoError(t, err)

	// Blind + closed: full data should be returned
	assert.Equal(t, 5, resp.TotalVotes, "blind closed poll should reveal TotalVotes")
	for _, opt := range resp.Options {
		if opt.ID == "opt-1" {
			assert.Equal(t, 3, opt.VoteCount)
			assert.Len(t, opt.Voters, 3)
		}
		if opt.ID == "opt-2" {
			assert.Equal(t, 2, opt.VoteCount)
			assert.Len(t, opt.Voters, 2)
		}
	}
}

func TestPollBuildResponse_StandardReturnsFullDataRegardless(t *testing.T) {
	pm, pollStore, tripModel, _ := setupPollModel()
	ctx := context.Background()

	tripModel.On("GetUserRole", ctx, "trip-1", "user-1").Return(types.MemberRoleMember, nil)

	standardPoll := &types.Poll{
		ID: "poll-1", TripID: "trip-1", Question: "Open vote",
		PollType: types.PollTypeStandard, IsBlind: false,
		Status: types.PollStatusActive, ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	pollStore.On("GetPoll", ctx, "poll-1", "trip-1").Return(standardPoll, nil)

	options := []*types.PollOption{
		{ID: "opt-1", PollID: "poll-1", Text: "A"},
	}
	pollStore.On("ListPollOptions", ctx, "poll-1").Return(options, nil)
	pollStore.On("GetVoteCountsByPoll", ctx, "poll-1").Return(map[string]int{"opt-1": 2}, nil)
	pollStore.On("ListVotesByPoll", ctx, "poll-1").Return([]*types.PollVote{
		{PollID: "poll-1", OptionID: "opt-1", UserID: "user-1"},
		{PollID: "poll-1", OptionID: "opt-1", UserID: "user-2"},
	}, nil)
	pollStore.On("GetUserVotesForPoll", ctx, "poll-1", "user-1").Return([]*types.PollVote{
		{PollID: "poll-1", OptionID: "opt-1", UserID: "user-1"},
	}, nil)

	resp, err := pm.GetPollWithResults(ctx, "trip-1", "poll-1", "user-1")
	require.NoError(t, err)

	assert.Equal(t, 2, resp.TotalVotes)
	assert.Equal(t, 2, resp.Options[0].VoteCount)
	assert.Len(t, resp.Options[0].Voters, 2)
}

// =============================================================================
// EVENT EMISSION TESTS
// =============================================================================

func TestPollClosePollWithEvent_EmitsPollClosed(t *testing.T) {
	pm, pollStore, tripModel, eventPub := setupPollModel()
	ctx := context.Background()

	expiredPoll := &types.Poll{
		ID: "poll-1", TripID: "trip-1", CreatedBy: "user-1",
		PollType: types.PollTypeStandard, IsBlind: false,
		Status: types.PollStatusActive, ExpiresAt: time.Now().Add(-1 * time.Hour), // expired
	}

	closedBy := "user-1"
	closedAt := time.Now()
	closedPoll := &types.Poll{
		ID: "poll-1", TripID: "trip-1", CreatedBy: "user-1",
		PollType: types.PollTypeStandard, IsBlind: false,
		Status: types.PollStatusClosed, ClosedBy: &closedBy, ClosedAt: &closedAt,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}

	pollStore.On("GetPoll", ctx, "poll-1", "trip-1").Return(expiredPoll, nil).Once()
	tripModel.On("GetUserRole", ctx, "trip-1", "user-1").Return(types.MemberRoleMember, nil)
	pollStore.On("ClosePoll", ctx, "poll-1", "trip-1", "user-1").Return(closedPoll, nil)
	stubBuildPollResponse(pollStore, "poll-1", []*types.PollOption{})

	// Expect exactly one Publish call with POLL_CLOSED
	eventPub.On("Publish", mock.Anything, "trip-1", mock.MatchedBy(func(e types.Event) bool {
		return e.Type == types.EventTypePollClosed
	})).Return(nil).Once()

	resp, err := pm.ClosePollWithEvent(ctx, "trip-1", "poll-1", "user-1")
	require.NoError(t, err)
	assert.NotNil(t, resp)

	eventPub.AssertNumberOfCalls(t, "Publish", 1)
}

func TestPollClosePollWithEvent_BlindEmitsClosedAndRevealed(t *testing.T) {
	pm, pollStore, tripModel, eventPub := setupPollModel()
	ctx := context.Background()

	expiredBlindPoll := &types.Poll{
		ID: "poll-1", TripID: "trip-1", CreatedBy: "user-1",
		PollType: types.PollTypeStandard, IsBlind: true,
		Status: types.PollStatusActive, ExpiresAt: time.Now().Add(-1 * time.Hour),
	}

	closedBy := "user-1"
	closedAt := time.Now()
	closedBlindPoll := &types.Poll{
		ID: "poll-1", TripID: "trip-1", CreatedBy: "user-1",
		PollType: types.PollTypeStandard, IsBlind: true,
		Status: types.PollStatusClosed, ClosedBy: &closedBy, ClosedAt: &closedAt,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}

	pollStore.On("GetPoll", ctx, "poll-1", "trip-1").Return(expiredBlindPoll, nil).Once()
	tripModel.On("GetUserRole", ctx, "trip-1", "user-1").Return(types.MemberRoleMember, nil)
	pollStore.On("ClosePoll", ctx, "poll-1", "trip-1", "user-1").Return(closedBlindPoll, nil)
	stubBuildPollResponse(pollStore, "poll-1", []*types.PollOption{})

	var publishedTypes []types.EventType
	eventPub.On("Publish", mock.Anything, "trip-1", mock.Anything).Run(func(args mock.Arguments) {
		evt := args.Get(2).(types.Event)
		publishedTypes = append(publishedTypes, evt.Type)
	}).Return(nil)

	resp, err := pm.ClosePollWithEvent(ctx, "trip-1", "poll-1", "user-1")
	require.NoError(t, err)
	assert.NotNil(t, resp)

	assert.Contains(t, publishedTypes, types.EventTypePollClosed, "should emit POLL_CLOSED")
	assert.Contains(t, publishedTypes, types.EventTypePollRevealed, "should emit POLL_REVEALED for blind poll")
	assert.Len(t, publishedTypes, 2, "blind poll close should emit exactly 2 events")
}

func TestPollCreatePollWithEvent_RichOptionsWithMetadata(t *testing.T) {
	pm, pollStore, tripModel, eventPub := setupPollModel()
	ctx := context.Background()

	imgURL := "https://example.com/beach.jpg"
	lat := 25.7617
	lng := -80.1918
	req := &types.PollCreate{
		Question: "Where next?",
		RichOptions: []types.PollOptionCreate{
			{Text: "Miami Beach", Metadata: &types.OptionMetadata{ImageURL: &imgURL, Lat: &lat, Lng: &lng}},
			{Text: "Central Park", Metadata: nil},
		},
		PollType: types.PollTypeStandard,
	}

	tripModel.On("GetUserRole", ctx, "trip-1", "user-1").Return(types.MemberRoleMember, nil)

	// Verify that CreatePollWithOptions receives options with metadata set
	pollStore.On("CreatePollWithOptions", ctx, mock.Anything, mock.MatchedBy(func(opts []*types.PollOption) bool {
		if len(opts) != 2 {
			return false
		}
		// First option should have metadata
		if opts[0].OptionMetadata == nil || opts[0].ImageURL == nil || *opts[0].ImageURL != imgURL {
			return false
		}
		if opts[0].Lat == nil || *opts[0].Lat != lat {
			return false
		}
		// Second option should have no metadata
		if opts[1].OptionMetadata != nil {
			return false
		}
		return true
	})).Return("poll-1", nil)

	pollStore.On("GetPoll", ctx, "poll-1", "trip-1").Return(&types.Poll{
		ID: "poll-1", TripID: "trip-1", Status: types.PollStatusActive, PollType: types.PollTypeStandard,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}, nil)
	stubBuildPollResponse(pollStore, "poll-1", []*types.PollOption{})

	eventPub.On("Publish", mock.Anything, "trip-1", mock.MatchedBy(func(e types.Event) bool {
		if e.Type != types.EventTypePollCreated {
			return false
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(e.Payload, &payload); err != nil {
			return false
		}
		return payload["question"] == "Where next?"
	})).Return(nil)

	resp, err := pm.CreatePollWithEvent(ctx, "trip-1", "user-1", req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	pollStore.AssertExpectations(t)
	eventPub.AssertExpectations(t)
}

func TestPollValidation_DuplicateOptionsForbiddenForStandard(t *testing.T) {
	pm, _, _, _ := setupPollModel()
	ctx := context.Background()

	req := &types.PollCreate{
		Question: "Duplicates?",
		Options:  []string{"Same", "same"},
		PollType: types.PollTypeStandard,
	}
	_, err := pm.CreatePollWithEvent(ctx, "trip-1", "user-1", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestPollValidation_TooFewOptions(t *testing.T) {
	pm, _, _, _ := setupPollModel()
	ctx := context.Background()

	req := &types.PollCreate{
		Question: "Only one?",
		Options:  []string{"Lonely"},
		PollType: types.PollTypeStandard,
	}
	_, err := pm.CreatePollWithEvent(ctx, "trip-1", "user-1", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 2 options")
}
