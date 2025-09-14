package errors_

import "errors"

var (
	ErrUserNotAvailable = errors.New("no user found with the provided username")
)
