package webapi

// region ErrorResponse ////////////////////////////////////////////////////////////////////////////////////////////////

// ErrorResponse is the response that is returned when an error occurred in any of the endpoints.
type ErrorResponse struct {
	Error string `json:"error"`
}

// NewErrorResponse returns an ErrorResponse from the given error.
func NewErrorResponse(err error) ErrorResponse {
	return ErrorResponse{
		Error: err.Error(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
