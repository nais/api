package loaderv1_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/nais/api/internal/v1/graphv1/loaderv1"
)

type mockDBModel struct {
	ID int
}

type mockGraphModel struct {
	ID string
}

func toGraphModel(dbModel *mockDBModel) *mockGraphModel {
	return &mockGraphModel{ID: strconv.Itoa(dbModel.ID)}
}

func TestLoadModels_happy(t *testing.T) {
	makeKey := func(m *mockGraphModel) int {
		id, _ := strconv.Atoi(m.ID)
		return id
	}

	loaderFunc := func(ctx context.Context, keys []int) ([]*mockDBModel, error) {
		ret := make([]*mockDBModel, len(keys))
		for i, key := range keys {
			ret[i] = &mockDBModel{ID: key}
		}

		return ret, nil
	}

	ret, errs := loaderv1.LoadModels(context.Background(), []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, loaderFunc, toGraphModel, makeKey)

	expectedErrs := make([]error, 10)
	if diff := cmp.Diff(expectedErrs, errs, cmpopts.EquateErrors()); diff != "" {
		t.Errorf("diff: -want +got\n%s", diff)
	}

	expected := []*mockGraphModel{
		{ID: "1"},
		{ID: "2"},
		{ID: "3"},
		{ID: "4"},
		{ID: "5"},
		{ID: "6"},
		{ID: "7"},
		{ID: "8"},
		{ID: "9"},
		{ID: "10"},
	}

	if diff := cmp.Diff(expected, ret); diff != "" {
		t.Errorf("diff: -want +got\n%s", diff)
	}
}

func TestLoadModels_some_missing(t *testing.T) {
	makeKey := func(m *mockGraphModel) int {
		id, _ := strconv.Atoi(m.ID)
		return id
	}

	loaderFunc := func(ctx context.Context, keys []int) ([]*mockDBModel, error) {
		ret := make([]*mockDBModel, len(keys))
		for i, key := range keys {
			if i > 5 {
				ret[i] = &mockDBModel{ID: key}
			}
		}

		return ret, nil
	}

	ret, errs := loaderv1.LoadModels(context.Background(), []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, loaderFunc, toGraphModel, makeKey)
	expectedErrs := []error{
		loaderv1.ErrObjectNotFound,
		loaderv1.ErrObjectNotFound,
		loaderv1.ErrObjectNotFound,
		loaderv1.ErrObjectNotFound,
		loaderv1.ErrObjectNotFound,
		loaderv1.ErrObjectNotFound,
		nil,
		nil,
		nil,
		nil,
	}
	if diff := cmp.Diff(expectedErrs, errs, cmpopts.EquateErrors()); diff != "" {
		t.Errorf("diff: -want +got\n%s", diff)
	}

	expected := []*mockGraphModel{
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		{ID: "7"},
		{ID: "8"},
		{ID: "9"},
		{ID: "10"},
	}

	if diff := cmp.Diff(expected, ret); diff != "" {
		t.Errorf("diff: -want +got\n%s", diff)
	}
}

func TestMiddleware(t *testing.T) {
	type ctxKeyType string
	const ctxKey ctxKeyType = "key"

	alterContext := func(ctx context.Context) context.Context {
		return context.WithValue(ctx, ctxKey, "value")
	}

	h := loaderv1.Middleware(alterContext)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Value(ctxKey) != "value" {
			t.Error("expected value in context")
		}
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
}
