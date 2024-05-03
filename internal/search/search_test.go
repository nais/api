package search_test

import (
	"context"
	"testing"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/search"
)

func TestSearcher_Search(t *testing.T) {
	ctx := context.Background()

	t.Run("empty result with no searchables", func(t *testing.T) {
		results := search.New().Search(ctx, "query", nil)
		if len(results) != 0 {
			t.Errorf("expected empty list, got %v", results)
		}
	})

	t.Run("search with multiple searchables", func(t *testing.T) {
		q := "some query"
		st := model.SearchType("type")
		filter := &model.SearchFilter{Type: &st}

		r1 := &search.Result{Node: &model.Team{Slug: "r1"}, Rank: 2}
		r2 := &search.Result{Node: &model.Team{Slug: "r2"}, Rank: 1}
		r3 := &search.Result{Node: &model.App{Name: "r3"}, Rank: 18}
		r4 := &search.Result{Node: &model.App{Name: "r4"}, Rank: 20}
		r5 := &search.Result{Node: &model.NaisJob{Name: "r5"}, Rank: 2}

		s1 := search.NewMockSearchable(t)
		s1.EXPECT().
			SupportsSearchFilter(filter).
			Return(false)

		s2 := search.NewMockSearchable(t)
		s2.EXPECT().
			SupportsSearchFilter(filter).
			Return(true)
		s2.EXPECT().
			Search(ctx, q, filter).
			Return([]*search.Result{r1, r2})

		s3 := search.NewMockSearchable(t)
		s3.EXPECT().
			SupportsSearchFilter(filter).
			Return(false)

		s4 := search.NewMockSearchable(t)
		s4.EXPECT().
			SupportsSearchFilter(filter).
			Return(true)
		s4.EXPECT().
			Search(ctx, q, filter).
			Return([]*search.Result{r3, r4, r5})

		results := search.New(s1, s2, s3, s4).Search(ctx, q, filter)
		if len(results) != 5 {
			t.Errorf("expected 4 results, got %v", results)
		}

		if results[0].Node.(*model.Team).Slug != "r2" {
			t.Errorf("expected %v, got %v", r2, results[0])
		}

		if results[1].Node.(*model.Team).Slug != "r1" {
			t.Errorf("expected %v, got %v", r1, results[0])
		}

		if results[2].Node.(*model.NaisJob).Name != "r5" {
			t.Errorf("expected %v, got %v", r5, results[0])
		}

		if results[3].Node.(*model.App).Name != "r3" {
			t.Errorf("expected %v, got %v", r3, results[0])
		}

		if results[4].Node.(*model.App).Name != "r4" {
			t.Errorf("expected %v, got %v", r4, results[0])
		}
	})
}
