package apperrors

import "errors"

var (
	ErrNotFound = errors.New("resource not found")
	ErrInternal = errors.New("internal server error")
)
