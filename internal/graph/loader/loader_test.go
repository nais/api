package loader

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/stretchr/testify/mock"
	"github.com/vikstrous/dataloadgen"
)

func TestGenericLoaders(t *testing.T) {
	ids := make([]uuid.UUID, 5)
	for i := range ids {
		ids[i] = uuid.New()
	}

	tests := map[string]struct {
		dbRet    []*database.User
		dbErr    error
		ids      []uuid.UUID
		want     []*model.User
		wantErrs []error
	}{
		"no users": {
			dbRet:    []*database.User{},
			dbErr:    nil,
			ids:      []uuid.UUID{uuid.New()},
			want:     []*model.User{nil},
			wantErrs: []error{pgx.ErrNoRows},
		},
		"one user": {
			dbRet: []*database.User{
				{
					User: &gensql.User{
						ID: ids[0],
					},
				},
			},
			dbErr:    nil,
			ids:      []uuid.UUID{ids[0]},
			want:     []*model.User{{ID: scalar.UserIdent(ids[0])}},
			wantErrs: []error{nil},
		},
		"five users": {
			dbRet: func() []*database.User {
				ret := make([]*database.User, 5)
				for i := range ret {
					ret[i] = &database.User{
						User: &gensql.User{
							ID: ids[i],
						},
					}
				}
				return ret
			}(),
			dbErr: nil,
			ids:   ids,
			want: func() []*model.User {
				ret := make([]*model.User, 5)
				for i := range ret {
					ret[i] = &model.User{ID: scalar.UserIdent(ids[i])}
				}
				return ret
			}(),
		},

		"five users, one missing": {
			dbRet: func() []*database.User {
				ret := make([]*database.User, 4)
				for i := range ret {
					ret[i] = &database.User{
						User: &gensql.User{
							ID: ids[i],
						},
					}
				}
				return ret
			}(),
			dbErr: nil,
			ids:   ids,
			want: func() []*model.User {
				ret := make([]*model.User, 5)
				for i := range ret {
					if i < 4 {
						ret[i] = &model.User{ID: scalar.UserIdent(ids[i])}
					}
				}
				return ret
			}(),
			wantErrs: []error{nil, nil, nil, nil, pgx.ErrNoRows},
		},

		"five users, all missing": {
			dbRet:    []*database.User{},
			dbErr:    nil,
			ids:      ids,
			want:     make([]*model.User, 5),
			wantErrs: []error{pgx.ErrNoRows, pgx.ErrNoRows, pgx.ErrNoRows, pgx.ErrNoRows, pgx.ErrNoRows},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			db := database.NewMockDatabase(t)
			db.EXPECT().GetUsersByIDs(mock.Anything, tc.ids).Once().Return(tc.dbRet, tc.dbErr)

			ctx := NewLoaderContext(context.Background(), db)
			got, errs := For(ctx).UserLoader.LoadAll(ctx, tc.ids)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("unexpected result (-want +got):\n%s", diff)
			}

			if errs != nil && !emptyErrSlice(tc.wantErrs) {
				errList := []error(errs.(dataloadgen.ErrorSlice))
				if diff := cmp.Diff(tc.wantErrs, errList, cmp.Comparer(func(a, b error) bool {
					return errors.Is(a, b)
				})); diff != "" {
					t.Errorf("unexpected errors (-want +got):\n%s", diff)
				}
			}

			if errs == nil && !emptyErrSlice(tc.wantErrs) {
				t.Errorf("expected errors, got nil")
			}
		})
	}
}

func emptyErrSlice(errs []error) bool {
	return len(errs) == 0 || slices.IndexFunc(errs, func(err error) bool { return err != nil }) == -1
}
