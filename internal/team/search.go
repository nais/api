package team

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/database/notify"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team/teamsql"
	"github.com/sirupsen/logrus"
)

func init() {
	search.Register("TEAM", func(ctx context.Context, q string) []*search.Result {
		ret, err := Search(ctx, q)
		if err != nil {
			return nil
		}
		return ret
	})
}

func AddSearch(client search.Client, pool *pgxpool.Pool, notifier *notify.Notifier, log logrus.FieldLogger) {
	client.AddClient("TEAM", &teamSearch{
		db:       teamsql.New(pool),
		notifier: notifier,
		log:      log,
	})
}

type teamSearch struct {
	log      logrus.FieldLogger
	notifier *notify.Notifier
	db       teamsql.Querier
}

func (t *teamSearch) Convert(ctx context.Context, ids ...ident.Ident) ([]search.SearchNode, error) {
	slugs := make([]slug.Slug, 0, len(ids))
	for _, id := range ids {
		slug, err := parseTeamIdent(id)
		if err != nil {
			return nil, err
		}
		slugs = append(slugs, slug)
	}

	all, err := t.db.ListBySlugs(ctx, slugs)
	if err != nil {
		return nil, err
	}

	ret := make([]search.SearchNode, 0, len(all))
	for _, team := range all {
		ret = append(ret, toGraphTeam(team))
	}

	return ret, nil
}

func (t *teamSearch) ReIndex(ctx context.Context) []search.Document {
	all, err := t.db.ListAllForSearch(ctx)
	if err != nil {
		return nil
	}

	ret := make([]search.Document, 0, len(all))
	for _, team := range all {
		ret = append(ret, newSearchDocument(team.Slug, team.Purpose))
	}

	return ret
}

func (t *teamSearch) Watch(ctx context.Context, indexer search.Indexer) error {
	go t.listen(ctx, indexer)
	return nil
}

func (t *teamSearch) listen(ctx context.Context, indexer search.Indexer) {
	ch := t.notifier.Listen("teams")

	for {
		select {
		case <-ctx.Done():
			return
		case payload := <-ch:
			data := dataFromNotification(payload)
			if data.Slug == "" {
				continue
			}

			switch payload.Op {
			case notify.Insert, notify.Update:
				if err := indexer.Update(newSearchDocument(data.Slug, data.Purpose)); err != nil {
					t.log.WithError(err).WithField("slug", data.Slug).Error("failed to update search index")
				}
			case notify.Delete:
				if err := indexer.Remove(newTeamIdent(data.Slug)); err != nil {
					t.log.WithError(err).WithField("slug", data.Slug).Error("failed to remove from search index")
				}
			default:
				t.log.WithField("op", payload.Op).Warn("unknown operation")
			}
		}
	}
}

type notificationData struct {
	Slug    slug.Slug `json:"slug"`
	Purpose string    `json:"purpose"`
}

func dataFromNotification(payload notify.Payload) notificationData {
	var slg slug.Slug
	var purpose string

	if sslug, ok := payload.Data["slug"].(string); ok {
		slg = slug.Slug(sslug)
	}

	if spurpose, ok := payload.Data["purpose"].(string); ok {
		purpose = spurpose
	}

	return notificationData{
		Slug:    slg,
		Purpose: purpose,
	}
}

func newSearchDocument(teamSlug slug.Slug, purpose string) search.Document {
	sslug := teamSlug.String()
	return search.Document{
		ID:   newTeamIdent(teamSlug).String(),
		Name: sslug,
		Team: sslug,
		Kind: "TEAM",
		Fields: map[string]string{
			"purpose": purpose,
		},
	}
}
