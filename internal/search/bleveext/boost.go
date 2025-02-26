package bleveext

import (
	"context"

	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/blevesearch/bleve/v2/search/searcher"
	index "github.com/blevesearch/bleve_index_api"
)

// Inspired by https://gist.github.com/rgalanakis/75dbea70f720d8393a0b83393a010836

// BoostingQueryPredicate is called with a search field, term,
// and whether the field was part of the match,
// or was included just for the purposes of the BoostingQuery.
//
// For example, maybe if any of 3 fields match,
// the document should be boosted.
// The predicate would be called with all three fields,
// but only those with isPartOfMatch should return a non-nil query.Boost.
//
// Additionally, perhaps some documents should be boosted
// merely because of their value ("sale" items).
// In this case, you would likely ignore isPartOfMatch
// and instead compare against term.
type BoostingQueryPredicate func(field string, term []byte, isPartOfMatch bool) *query.Boost

type BoostingQuery struct {
	// BoostVal represents the ratio of recency to preexisitng score. The
	// default, 1.0, assigns equal importance to recency score and match score.
	// A value of 2 would relatively rank recency twice as important as match score.
	BoostVal *query.Boost
	// These fields are loaded, and used in the boost processing.
	// Predicate will be called with every field.
	Fields []string
	// Return true if BoostVal should be applied to this hit.
	Predicate BoostingQueryPredicate
	base      query.Query
}

func NewBoostingQuery(base query.Query, fields []string, predicate BoostingQueryPredicate) *BoostingQuery {
	return &BoostingQuery{Fields: fields, Predicate: predicate, base: base}
}

// SetBoost sets the boost value.
// Usually you should leave this, and return differing values from Predicate.
// Changing this scales the result of Predicate.
func (q *BoostingQuery) SetBoost(b float64) {
	boost := query.Boost(b)
	q.BoostVal = &boost
}

func (q *BoostingQuery) Boost() float64 {
	return q.BoostVal.Value()
}

func (q *BoostingQuery) Searcher(ctx context.Context, i index.IndexReader, m mapping.IndexMapping, options search.SearcherOptions) (search.Searcher, error) {
	bs, err := q.base.Searcher(ctx, i, m, options)
	if err != nil {
		return nil, err
	}
	dvReader, err := i.DocValueReader(q.Fields)
	if err != nil {
		return nil, err
	}
	return searcher.NewFilteringSearcher(ctx, bs, q.makeFilter(dvReader)), nil
}

func (q *BoostingQuery) makeFilter(dvReader index.DocValueReader) searcher.FilterFunc {
	boost := q.Boost()
	return func(d *search.DocumentMatch) bool {
		isPartOfMatch := make(map[string]bool, len(d.FieldTermLocations))
		for _, ftloc := range d.FieldTermLocations {
			isPartOfMatch[ftloc.Field] = true
		}
		seenFields := make(map[string]struct{}, len(d.Fields))
		_ = dvReader.VisitDocValues(d.IndexInternalID, func(field string, term []byte) {
			if _, seen := seenFields[field]; seen {
				return
			}
			seenFields[field] = struct{}{}
			b := q.Predicate(field, term, isPartOfMatch[field])
			if b != nil {
				d.Score *= boost * b.Value()
			}
		})
		return true
	}
}
