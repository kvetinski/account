package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidNick        = errors.New("invalid nick")
	ErrInvalidPhone       = errors.New("invalid phone")
	ErrNickAlreadyExists  = errors.New("nick already exists")
	ErrPhoneAlreadyExists = errors.New("phone already exists")
	ErrAccountNotFound    = errors.New("account not found")
)

type Account struct {
	ID        uuid.UUID  `json:"id"`
	Nick      string     `json:"nick"`
	Phone     string     `json:"phone"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}
