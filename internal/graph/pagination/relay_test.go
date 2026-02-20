package pagination_test

import (
	"testing"

	"github.com/nais/api/internal/graph/pagination"
)

func TestParsePage(t *testing.T) {
	type args struct {
		first  *int
		after  *pagination.Cursor
		last   *int
		before *pagination.Cursor
	}

	tests := map[string]struct {
		args       args
		wantOffset int32
		wantLimit  int32
		errMsg     string
	}{
		"no values": {
			args:       args{},
			errMsg:     "",
			wantOffset: 0,
			wantLimit:  20,
		},

		"first and last": {
			args: args{
				first: new(123),
				last:  new(123),
			},
			errMsg:     "last must be used with before",
			wantOffset: 0,
			wantLimit:  20,
		},

		"first and before": {
			args: args{
				first: new(123),
				before: &pagination.Cursor{
					Offset: 0,
				},
			},
			errMsg:     "first and before cannot be used together",
			wantOffset: 0,
			wantLimit:  20,
		},

		"last and after": {
			args: args{
				last: new(123),
				after: &pagination.Cursor{
					Offset: 0,
				},
			},
			errMsg:     "last and after cannot be used together",
			wantOffset: 0,
			wantLimit:  20,
		},

		"invalid first": {
			args: args{
				first: new(0),
			},
			errMsg:     "first must be greater than or equal to 1",
			wantOffset: 0,
			wantLimit:  20,
		},

		"invalid last": {
			args: args{
				last: new(0),
				before: &pagination.Cursor{
					Offset: 0,
				},
			},
			errMsg:     "last must be greater than or equal to 1",
			wantOffset: 0,
			wantLimit:  20,
		},

		"valid first case": {
			args: args{
				first: new(10),
				after: &pagination.Cursor{
					Offset: 10,
				},
			},
			errMsg:     "",
			wantOffset: 11,
			wantLimit:  10,
		},

		"valid last case": {
			args: args{
				last: new(2),
				before: &pagination.Cursor{
					Offset: 7,
				},
			},
			errMsg:     "",
			wantOffset: 5,
			wantLimit:  2,
		},

		"negative offset": {
			args: args{
				last: new(5),
				before: &pagination.Cursor{
					Offset: 2,
				},
			},
			errMsg:     "",
			wantOffset: 0,
			wantLimit:  5,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := pagination.ParsePage(tt.args.first, tt.args.after, tt.args.last, tt.args.before)
			if tt.errMsg == "" && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Fatalf("Expected error message %q, got: %q", tt.errMsg, err)
			}

			if got.Limit() != tt.wantLimit {
				t.Errorf("Expected limit %d, got: %d", tt.wantLimit, got.Limit())
			}

			if got.Offset() != tt.wantOffset {
				t.Errorf("Expected offset %d, got %d", tt.wantOffset, got.Offset())
			}
		})
	}
}
