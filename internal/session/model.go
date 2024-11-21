package session

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID      uuid.UUID
	UserID  uuid.UUID
	Expires time.Time
}

func (s *Session) HasExpired() bool {
	return s.Expires.Before(time.Now())
}
