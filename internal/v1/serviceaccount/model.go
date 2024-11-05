package serviceaccount

import (
	"github.com/google/uuid"
)

type ServiceAccount struct {
	UUID uuid.UUID
	Name string
}

func (s *ServiceAccount) GetID() uuid.UUID       { return s.UUID }
func (s *ServiceAccount) Identity() string       { return s.Name }
func (s *ServiceAccount) IsServiceAccount() bool { return true }
