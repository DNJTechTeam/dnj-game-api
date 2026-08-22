package errors

type APIServiceError struct {
	Status  int
	Code    string
	Message string
	Details any
}

func (e *APIServiceError) Error() string { return e.Message }

func NewAPIServiceError(status int, code, message string, details any) *APIServiceError {
	return &APIServiceError{Status: status, Code: code, Message: message, Details: details}
}
