package types

import "time"

type PollStatus string

const (
	PollStatusActive PollStatus = "ACTIVE"
	PollStatusClosed PollStatus = "CLOSED"
)

type Poll struct {
	ID                 string     `json:"id"`
	TripID             string     `json:"tripId"`
	Question           string     `json:"question"`
	Status             PollStatus `json:"status"`
	AllowMultipleVotes bool       `json:"allowMultipleVotes"`
	CreatedBy          string     `json:"createdBy"`
	ClosedBy           *string    `json:"closedBy,omitempty"`
	ClosedAt           *time.Time `json:"closedAt,omitempty"`
	ExpiresAt          time.Time  `json:"expiresAt"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

func (p *Poll) IsExpired() bool {
	return time.Now().After(p.ExpiresAt)
}

type PollOption struct {
	ID        string    `json:"id"`
	PollID    string    `json:"pollId"`
	Text      string    `json:"text"`
	Position  int       `json:"position"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
}

type PollVote struct {
	ID        string    `json:"id"`
	PollID    string    `json:"pollId"`
	OptionID  string    `json:"optionId"`
	UserID    string    `json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
}

// API request types

type PollCreate struct {
	Question           string   `json:"question" binding:"required,max=500"`
	Options            []string `json:"options" binding:"required,min=2,max=20,dive,min=1,max=200"`
	AllowMultipleVotes bool     `json:"allowMultipleVotes"`
	DurationMinutes    *int     `json:"durationMinutes,omitempty"`
}

type PollUpdate struct {
	Question *string `json:"question,omitempty" binding:"omitempty,min=1,max=500"`
}

type CastVoteRequest struct {
	OptionID string `json:"optionId" binding:"required"`
}

// API response types (fat responses)

type PollVoter struct {
	UserID    string    `json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
}

type PollOptionWithVotes struct {
	PollOption
	VoteCount int         `json:"voteCount"`
	Voters    []PollVoter `json:"voters"`
	HasVoted  bool        `json:"hasVoted"`
}

type PollResponse struct {
	Poll
	Options       []PollOptionWithVotes `json:"options"`
	TotalVotes    int                   `json:"totalVotes"`
	UserVoteCount int                   `json:"userVoteCount"`
}
