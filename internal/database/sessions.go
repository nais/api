package database

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/database/gensql"
)

const sessionLength = 30 * time.Minute

type SessionRepo interface {
	CreateSession(ctx context.Context, userID uuid.UUID) (*Session, error)
	DeleteSession(ctx context.Context, sessionID uuid.UUID) error
	ExtendSession(ctx context.Context, sessionID uuid.UUID) (*Session, error)
	GetSessionByID(ctx context.Context, sessionID uuid.UUID) (*Session, error)
}

type Session struct {
	*gensql.Session
}

func (d *database) CreateSession(ctx context.Context, userID uuid.UUID) (*Session, error) {
	session, err := d.querier.CreateSession(ctx, userID, pgtype.Timestamptz{Time: time.Now().Add(sessionLength), Valid: true})
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
	session, err := d.querier.SetSessionExpires(ctx, pgtype.Timestamptz{Time: time.Now().Add(sessionLength), Valid: true}, sessionID)
	if err != nil {
		return nil, err
	}

	return &Session{Session: session}, nil
}
