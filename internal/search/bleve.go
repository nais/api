package search

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/search/bleveext"
	"github.com/nais/api/internal/search/searchsql"
	"github.com/sirupsen/logrus"
)

type Document struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Team   string            `json:"team,omitempty"`
	Kind   string            `json:"kind"`
	Fields map[string]string `json:"fields,omitempty"`
}

func (m Document) Type() string {
	return "doc"
}

type Indexer interface {
	Update(doc Document) error
	Remove(id ident.Ident) error
}

type Searchable interface {
	Convert(ctx context.Context, ids ...ident.Ident) ([]SearchNode, error)
	ReIndex(ctx context.Context) []Document
	Watch(ctx context.Context, indexer Indexer) error
}

type Client interface {
	Search(ctx context.Context, page *pagination.Pagination, filter SearchFilter) (*SearchNodeConnection, error)
	AddClient(kind SearchType, client Searchable)
	ReIndex(ctx context.Context) error
}

func buildIndexMapping() (mapping.IndexMapping, error) {
	indexMapping := bleve.NewIndexMapping()

	docMapping := bleve.NewDocumentMapping()
	docMapping.AddFieldMappingsAt("kind", bleve.NewKeywordFieldMapping())
	indexMapping.AddDocumentMapping("doc", docMapping)

	err := indexMapping.AddCustomAnalyzer(custom.Name,
		map[string]any{
			"type":      "custom",
			"tokenizer": `unicode`,
		})
	if err != nil {
		return nil, err
	}

	return indexMapping, nil
}

func New(ctx context.Context, pool *pgxpool.Pool, log logrus.FieldLogger) (Client, error) {
	im, err := buildIndexMapping()
	if err != nil {
		return nil, err
	}
	// im.CustomAnalysis = custom.AnalyzerConstructor(config map[string]interface{}, cache *registry.Cache)
	bleveIndex, err := bleve.NewMemOnly(im)
	if err != nil {
		return nil, err
	}

	bleveSearch := &bleveSearcher{
		Client:  bleveIndex,
		Clients: make(map[SearchType]Searchable),
		log:     log,
		db:      searchsql.New(pool),
	}

	return bleveSearch, nil
}

type bleveSearcher struct {
	Client  bleve.Index
	Clients map[SearchType]Searchable

	log logrus.FieldLogger
	db  searchsql.Querier
}

func (b *bleveSearcher) AddClient(kind SearchType, client Searchable) {
	b.Clients[kind] = client
}

func (b *bleveSearcher) ReIndex(ctx context.Context) error {
	b.reindexAll(ctx)
	for kind, search := range b.Clients {
		if err := search.Watch(ctx, b); err != nil {
			return fmt.Errorf("failed to watch %q: %w", kind, err)
		}
	}
	return nil
}

func (b *bleveSearcher) Update(doc Document) error {
	b.log.WithField("id", doc).Debug("indexing document")

	return b.Client.Index(doc.ID, doc)
}

func (b *bleveSearcher) Remove(id ident.Ident) error {
	b.log.WithField("id", id).Debug("removing document")

	return b.Client.Delete(id.String())
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

func (b *bleveSearcher) Search(ctx context.Context, page *pagination.Pagination, filter SearchFilter) (*SearchNodeConnection, error) {
	slugs, err := b.db.TeamSlugsFromUserID(ctx, authz.ActorFromContext(ctx).User.GetID())
	if err != nil {
		b.log.WithError(err).Error("failed to list teams")
		return nil, err
	}

	if !slices.Contains(slugs, "nais") {
		return nil, nil
	}

	queries := []query.Query{}

	if filter.Query != "" {
		qq := bleve.NewFuzzyQuery(filter.Query)
		qq.SetFuzziness(2)
		qq.SetBoost(0.5)

		prefix := bleve.NewPrefixQuery(filter.Query)
		prefix.SetField("name")
		prefix.SetBoost(1.5)

		// We add the query with both a match, prefix, and a fuzzy query to get both exact and fuzzy matches
		queries = append(queries, bleve.NewDisjunctionQuery(
			prefix,
			bleve.NewMatchQuery(filter.Query),
			qq,
		))
	}

	if filter.Type != nil {
		kind := bleve.NewTermQuery(filter.Type.String())
		kind.FieldVal = "kind"
		queries = append(queries, kind)
	}
	var q query.Query = bleve.NewConjunctionQuery(queries...)

	if len(slugs) > 0 {
		teamSlugs := make([]string, 0, len(slugs))
		for _, slug := range slugs {
			teamSlugs = append(teamSlugs, slug.String())
		}

		q = bleveext.NewBoostingQuery(q, []string{"team"}, func(field string, term []byte, isPartOfMatch bool) *query.Boost {
			v := 1
			if slices.Contains(teamSlugs, string(term)) {
				v = 1000
			}
			b := query.Boost(v)
			return &b
		})
	}

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
			continue
		}

		ret = append(ret, n)
	}

	return pagination.NewConnection(ret, page, results.Total), nil
}
