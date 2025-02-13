package types

import (
    "encoding/json"
    "time"
)

// TripAuditLog represents an audit log entry for trip-related events
type TripAuditLog struct {
    ID            string          `json:"id"`
    TripID        string          `json:"tripId"`
    EventType     EventType       `json:"eventType"`
    UserID        string          `json:"userId"`
    Timestamp     time.Time       `json:"timestamp"`
    Details       json.RawMessage `json:"details"`
    PreviousState json.RawMessage `json:"previousState,omitempty"`
    NewState      json.RawMessage `json:"newState,omitempty"`
    Metadata      json.RawMessage `json:"metadata,omitempty"`
}

// ListAuditLogsParams extends PaginationParams with audit-specific filters
type ListAuditLogsParams struct {
    PaginationParams
    TripID    string    `form:"tripId" binding:"required"`
    EventType EventType `form:"eventType"`
    FromDate  time.Time `form:"fromDate"`
    ToDate    time.Time `form:"toDate"`
    UserID    string    `form:"userId"`
}

// AuditLogSummary represents a summary of audit logs for a trip
type AuditLogSummary struct {
    TotalEvents         int                 `json:"totalEvents"`
    EventTypeBreakdown  map[EventType]int   `json:"eventTypeBreakdown"`
    LastEventTimestamp  time.Time           `json:"lastEventTimestamp"`
}