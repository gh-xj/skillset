package appctx

import "errors"

const (
	ExitSuccess = 0
	ExitError   = 1
	ExitUsage   = 2
)

type ExitCodeError struct {
	Code    int
	Message string
}

func NewExitError(code int, message string) *ExitCodeError {
	return &ExitCodeError{Code: code, Message: message}
}

func (e *ExitCodeError) Error() string {
	return e.Message
}

func ResolveExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}
	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return ExitError
}
