package search

import (
	"context"
	"fmt"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/sirupsen/logrus"
	"k8s.io/utils/ptr"
)

type Document struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Team   string            `json:"team,omitempty"`
	Kind   string            `json:"kind"`
	Fields map[string]string `json:"fields,omitempty"`
}

type Searchable interface {
	Convert(ctx context.Context, ids ...ident.Ident) ([]SearchNode, error)
	ReIndex(ctx context.Context) []Document
}

var (
	bleveSearches = map[SearchType]Searchable{}
	bleveSearch   *bleveSearcher
)

func RegisterBleve(searchType SearchType, search Searchable) {
	bleveSearches[searchType] = search
}

func InitBleve(ctx context.Context, log logrus.FieldLogger) error {
	bleveIndex, err := bleve.NewMemOnly(bleve.NewIndexMapping())
	if err != nil {
		return err
	}

	bleveSearch = &bleveSearcher{
		Client:  bleveIndex,
		Clients: bleveSearches,
		log:     log,
	}

	bleveSearch.reindexAll(ctx)

	return nil
}

type bleveSearcher struct {
	Client  bleve.Index
	Clients map[SearchType]Searchable

	log logrus.FieldLogger
}

func (b *bleveSearcher) reindexAll(ctx context.Context) {
	wg := &sync.WaitGroup{}
	for typ, search := range b.Clients {
		wg.Add(1)
		go func(search Searchable) {
			defer wg.Done()
			if err := b.index(typ, search.ReIndex(ctx)); err != nil {
				b.log.WithField("search_type", typ).WithError(err).Error("failed to reindex")
			}
		}(search)
	}
	wg.Wait()
}

func (b *bleveSearcher) index(typ SearchType, docs []Document) error {
	batch := b.Client.NewBatch()
	for _, doc := range docs {
		doc.Kind = typ.String()
		if err := batch.Index(doc.ID, doc); err != nil {
			return err
		}
	}

	return b.Client.Batch(batch)
}

func (b *bleveSearcher) search(ctx context.Context, page *pagination.Pagination, filter SearchFilter) (*SearchNodeConnection, error) {
	slugs, err := db(ctx).TeamSlugsFromUserID(ctx, authz.ActorFromContext(ctx).User.GetID())
	if err != nil {
		b.log.WithError(err).Error("failed to list teams")
		return nil, err
	}

	queries := []query.Query{}

	if filter.Query != "" {
		queries = append(queries, bleve.NewMatchQuery(filter.Query))
	}
	if len(slugs) > 0 {
		teamQueries := make([]query.Query, 0, len(slugs))
		for _, slug := range slugs {
			tq := bleve.NewTermQuery(slug.String())
			tq.SetField("team")
			tq.BoostVal = ptr.To[query.Boost](1000.0)
			teamQueries = append(teamQueries, tq)
		}

		teamFilter := bleve.NewDisjunctionQuery(teamQueries...)
		teamFilter.SetMin(0)
		queries = append(queries, teamFilter)
	}

	if filter.Type != nil {
		kind := bleve.NewMatchQuery(filter.Type.String())
		kind.FieldVal = "kind"
		queries = append(queries, kind)
	}
	q := bleve.NewConjunctionQuery(queries...)

	search := bleve.NewSearchRequest(q)
	search.Size = int(page.Limit())
	search.From = int(page.Offset())
	search.Fields = []string{"kind"}

	results, err := b.Client.Search(search)
	if err != nil {
		b.log.WithError(err).Error("bleve search failed")
		return nil, err
	}

	b.log.WithFields(logrus.Fields{
		"duration": results.Took,
		"total":    results.Total,
		"hits":     len(results.Hits),
	}).Debug("search results")

	kinds := map[string][]ident.Ident{}
	for _, hit := range results.Hits {
		kind := hit.Fields["kind"].(string)
		kinds[kind] = append(kinds[kind], ident.FromString(hit.ID))
	}

	convertedResults := make(map[string]SearchNode)
	for kind, ids := range kinds {
		client, ok := b.Clients[SearchType(kind)]
		if !ok {
			b.log.WithField("kind", kind).Error("missing search client")
			continue
		}
		sn, err := client.Convert(ctx, ids...)
		if err != nil {
			b.log.WithError(err).Error("failed to convert search results")
			continue
		}

		for _, n := range sn {
			convertedResults[n.ID().String()] = n
		}
	}

	ret := make([]SearchNode, 0, len(results.Hits))
	for _, hit := range results.Hits {
		n, ok := convertedResults[hit.ID]
		if !ok {
			b.log.WithField("id", hit.ID).Error("missing search result")
			return nil, fmt.Errorf("missing %v from search result", hit.ID)
		}

		ret = append(ret, n)
	}

	return pagination.NewConnection(ret, page, results.Total), nil
}
