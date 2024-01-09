package database

import (
	"context"
	"time"

	"github.com/google/uuid"
	sqlc "github.com/nais/api/internal/database/gensql"
)

const sessionLength = 30 * time.Minute

func (d *database) CreateSession(ctx context.Context, userID uuid.UUID) (*Session, error) {
	session, err := d.querier.CreateSession(ctx, sqlc.CreateSessionParams{
		UserID:  userID,
		Expires: time.Now().Add(sessionLength),
	})
	if err != nil {
		return nil, err
	}

	return &Session{Session: session}, nil
}

func (d *database) DeleteSession(ctx context.Context, sessionID uuid.UUID) error {
	return d.querier.DeleteSession(ctx, sessionID)
}

func (d *database) GetSessionByID(ctx context.Context, sessionID uuid.UUID) (*Session, error) {
	session, err := d.querier.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return &Session{Session: session}, nil
}

func (d *database) ExtendSession(ctx context.Context, sessionID uuid.UUID) (*Session, error) {
	session, err := d.querier.SetSessionExpires(ctx, sqlc.SetSessionExpiresParams{
		Expires: time.Now().Add(sessionLength),
		ID:      sessionID,
	})
	if err != nil {
		return nil, err
	}

	return &Session{Session: session}, nil
}
