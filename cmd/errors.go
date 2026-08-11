package cmd

// UsageError maps to process exit code 2.
type UsageError struct {
	msg string
}

func (e *UsageError) Error() string {
	return e.msg
}

func NewUsageError(msg string) *UsageError {
	return &UsageError{msg: msg}
}
