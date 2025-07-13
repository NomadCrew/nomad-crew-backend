package types

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Pagination struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
		Total  int `json:"total"`
	} `json:"pagination"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// PaginationParams defines common pagination query parameters
type PaginationParams struct {
	Limit  int `form:"limit" binding:"omitempty,gte=0"`
	Offset int `form:"offset" binding:"omitempty,gte=0"`
}

type ListTodosParams struct {
	Limit  int `form:"limit,default=20"`
	Offset int `form:"offset,default=0"`
}

type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

// ChatMessagesResponse defines the response structure for chat messages endpoint
type ChatMessagesResponse struct {
	Messages   []ChatMessage      `json:"messages"`
	Pagination ChatPaginationInfo `json:"pagination"`
}

// ChatPaginationInfo defines pagination info for chat messages
type ChatPaginationInfo struct {
	HasMore    bool    `json:"has_more"`
	NextCursor *string `json:"next_cursor,omitempty"`
	Limit      int     `json:"limit"`
	Before     string  `json:"before,omitempty"`
}

// LocationsResponse defines the response structure for locations endpoint
type LocationsResponse struct {
	Locations  []MemberLocation   `json:"locations"`
	Pagination LocationPagination `json:"pagination"`
}

// LocationPagination defines pagination info for locations
type LocationPagination struct {
	HasMore bool `json:"has_more"`
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
}
