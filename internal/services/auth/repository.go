package auth

import (
	"context"
	"errors"
)

// ErrDuplicate is returned when trying to create a user with an email that already exists
var ErrDuplicate = errors.New("user with this email already exists")

// UsersRepo defines the interface for user repository operations
type UsersRepo interface {
	Create(ctx context.Context, user *User) error
	FindByEmail(ctx context.Context, email string) (*User, error)
}
