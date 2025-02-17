package session

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/session/sessionsql"
)

const (
	sessionLength    = time.Minute * 30
	maxSessionLength = time.Hour * 8
)

func Create(ctx context.Context, userID uuid.UUID) (*Session, error) {
	r, err := db(ctx).Create(ctx, sessionsql.CreateParams{
		UserID: userID,
		Expires: pgtype.Timestamptz{
			Time:  time.Now().Add(sessionLength),
			Valid: true,
		},
	})
	if err != nil {
		return nil, err
	}
	return &Session{
		ID:      r.ID,
		UserID:  r.UserID,
		Expires: r.Expires.Time,
	}, nil
}

func Get(ctx context.Context, sessionID uuid.UUID) (*Session, error) {
	r, err := db(ctx).Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return &Session{
		ID:        r.ID,
		UserID:    r.UserID,
		Expires:   r.Expires.Time,
		CreatedAt: r.CreatedAt.Time,
	}, nil
}

func Delete(ctx context.Context, sessionID uuid.UUID) error {
	return db(ctx).Delete(ctx, sessionID)
}

func Extend(ctx context.Context, sessionID uuid.UUID) (*Session, error) {
	r, err := db(ctx).SetExpires(ctx, sessionsql.SetExpiresParams{
		Expires: pgtype.Timestamptz{
			Time:  time.Now().Add(sessionLength),
			Valid: true,
		},
		ID: sessionID,
	})
	if err != nil {
		return nil, err
	}
	return &Session{
		ID:      r.ID,
		UserID:  r.UserID,
		Expires: r.Expires.Time,
	}, nil
}
