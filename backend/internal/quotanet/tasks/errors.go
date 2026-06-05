package tasks

import "errors"

var (
	ErrTaskNotFound          = errors.New("quotanet task not found")
	ErrDuplicateTaskResponse = errors.New("quotanet duplicate task response")
)
