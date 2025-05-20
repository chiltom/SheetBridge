package apperrors

import "fmt"

// Standard application errors
var (
	ErrNotFound       = New("resource_not_found", "The requested resource could not be found.")
	ErrInvalidInput   = New("invalid_input", "The input provided is invalid.")
	ErrDatabase       = New("database_error", "A database error occurred.")
	ErrCSVProcessing  = New("csv_processing_error", "An error occurred while processing the CSV file.")
	ErrFileOperation  = New("file_operation_error", "An error occurred during a file operation.")
	ErrInternalServer = New("internal_server_error", "An unexpected error occurred on the server.")
	ErrDataConflict   = New("data_conflict", "The operation could not be completed due to a data conflict (e.g., table exists).")
	ErrTypeConversion = New("type_conversion_error", "Failed to convert data to the target type.")
)

// AppError defines a standard application error
type AppError struct {
	Code    string `json:"code"`    // Machine-readable error code
	Message string `json:"message"` // Human-readable message
	Err     error  `json:"-"`       // Underlying error, not exposed in JSON by default
}

// Error builds and returns an error string
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s (code: %s, original_error: %v)", e.Message, e.Code, e.Err)
	}
	return fmt.Sprintf("%s (code: %s)", e.Message, e.Code)
}

// New creates a new AppError
func New(code, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

// Wrap wraps an existing error with an AppError, providing additional context
// If customMessage is provided, it overrides the default message of appErr
func Wrap(err error, baseAppErr *AppError, customMessage ...string) *AppError {
	msg := baseAppErr.Message
	if len(customMessage) > 0 && customMessage[0] != "" {
		msg = customMessage[0]
	}
	return &AppError{
		Code:    baseAppErr.Code,
		Message: msg,
		Err:     err,
	}
}

// Is checks if an error is of a specific AppError type by comparing codes
func Is(err error, target *AppError) bool {
	if e, ok := err.(*AppError); ok {
		return e.Code == target.Code
	}
	return false
}
