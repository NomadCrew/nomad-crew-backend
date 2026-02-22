package types

import (
	"encoding/json"
	"time"
)

type PollStatus string

const (
	PollStatusActive PollStatus = "ACTIVE"
	PollStatusClosed PollStatus = "CLOSED"
)

type PollType string

const (
	PollTypeStandard   PollType = "standard"
	PollTypeBinary     PollType = "binary"
	PollTypeEmoji      PollType = "emoji"
	PollTypeSchedule   PollType = "schedule"
	PollTypeVibeCheck  PollType = "vibe_check"
)

type Poll struct {
	ID                 string     `json:"id"`
	TripID             string     `json:"tripId"`
	Question           string     `json:"question"`
	PollType           PollType   `json:"pollType"`
	IsBlind            bool       `json:"isBlind"`
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

// OptionMetadata holds optional rich data for poll options (stored as JSONB).
type OptionMetadata struct {
	ImageURL *string  `json:"imageUrl,omitempty"`
	Lat      *float64 `json:"lat,omitempty"`
	Lng      *float64 `json:"lng,omitempty"`
}

type PollOption struct {
	ID             string           `json:"id"`
	PollID         string           `json:"pollId"`
	Text           string           `json:"text"`
	Position       int              `json:"position"`
	CreatedBy      string           `json:"createdBy"`
	CreatedAt      time.Time        `json:"createdAt"`
	OptionMetadata *OptionMetadata  `json:"optionMetadata,omitempty"`
	// Convenience fields flattened from OptionMetadata for API responses
	ImageURL       *string          `json:"imageUrl,omitempty"`
	Lat            *float64         `json:"lat,omitempty"`
	Lng            *float64         `json:"lng,omitempty"`
}

// UnmarshalOptionMetadata parses the JSONB option_metadata column into the flattened fields.
func (o *PollOption) UnmarshalOptionMetadata(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	var meta OptionMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}
	o.OptionMetadata = &meta
	o.ImageURL = meta.ImageURL
	o.Lat = meta.Lat
	o.Lng = meta.Lng
	return nil
}

type PollVote struct {
	ID        string    `json:"id"`
	PollID    string    `json:"pollId"`
	OptionID  string    `json:"optionId"`
	UserID    string    `json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
}

// API request types

type PollOptionCreate struct {
	Text     string          `json:"text" binding:"required,max=200"`
	Metadata *OptionMetadata `json:"metadata,omitempty"`
}

type PollCreate struct {
	Question           string             `json:"question" binding:"required,max=500"`
	Options            []string           `json:"options,omitempty" binding:"omitempty,min=2,max=20,dive,min=1,max=200"`
	RichOptions        []PollOptionCreate `json:"richOptions,omitempty"`
	PollType           PollType           `json:"pollType,omitempty"`
	IsBlind            bool               `json:"isBlind,omitempty"`
	AllowMultipleVotes bool               `json:"allowMultipleVotes"`
	DurationMinutes    *int               `json:"durationMinutes,omitempty"`
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
