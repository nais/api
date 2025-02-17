package session

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Expires   time.Time
	CreatedAt time.Time
}

func (s *Session) HasExpired() bool {
	if time.Since(s.CreatedAt) > maxSessionLength {
		return true
	}

	return s.Expires.Before(time.Now())
}
