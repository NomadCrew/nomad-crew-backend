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
