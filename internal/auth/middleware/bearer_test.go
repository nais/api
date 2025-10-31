// Borrowed and adapted from https://github.com/golang/go/issues/70522

package middleware

import (
	"net/http/httptest"
	"testing"
)

func TestBearerAuth(t *testing.T) {
	cases := []struct {
		name, in, out string
		ok            bool
	}{
		{"valid 1-char alpha", "Q", "Q", true},
		{"valid alphanum", "QWxhZGRpbjpvcGVuIHNlc2FtZQ==", "QWxhZGRpbjpvcGVuIHNlc2FtZQ==", true},
		{"valid alphanum and others", "QWxhZGRpbjpvcGVuIHNlc2FtZQ-._~+/==", "QWxhZGRpbjpvcGVuIHNlc2FtZQ-._~+/==", true},
		{"empty", "", "", false},
	}
	for _, tt := range cases {
		f := func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://example.com/", nil)
			r.Header.Set("Authorization", "Bearer "+tt.in)

			token, ok := BearerAuth(r)
			if tt.ok != ok || token != tt.out {
				t.Errorf("BearerAuth(): got %q, %t, want %q, %t", token, ok, tt.out, tt.ok)
			}
		}
		t.Run(tt.name, f)
	}
	f := func(t *testing.T) {
		// Unauthenticated request.
		r := httptest.NewRequest("GET", "http://example.com/", nil)
		const (
			wantToken = ""
			wantOk    = false
		)
		token, ok := BearerAuth(r)
		if ok != wantOk || token != wantToken {
			t.Errorf("BearerAuth(): got %q, %t, want %q, %t", token, ok, wantToken, wantOk)
		}
	}
	t.Run("unauthenticated", f)
}

func TestParseBearerAuth(t *testing.T) {
	cases := []struct {
		name, header, token string
		ok                  bool
	}{
		{"valid", "Bearer QWxhZGRpbjpvcGVuIHNlc2FtZQ==", "QWxhZGRpbjpvcGVuIHNlc2FtZQ==", true},

		// Case of authentication scheme doesn't matter:
		{"uppercase", "BEARER QWxhZGRpbjpvcGVuIHNlc2FtZQ==", "QWxhZGRpbjpvcGVuIHNlc2FtZQ==", true},
		{"lowercase", "bearer QWxhZGRpbjpvcGVuIHNlc2FtZQ==", "QWxhZGRpbjpvcGVuIHNlc2FtZQ==", true},

		{"missing space", "BearerQWxhZGRpbjpvcGVuIHNlc2FtZQ==", "", false},
		{"missing token", "Bearer ", "", false},
		{"missing Bearer", "QWxhZGRpbjpvcGVuIHNlc2FtZQ==", "", false},
		{"non-Bearer auth scheme", `Digest username="Aladdin"`, "", false},
	}
	for _, tt := range cases {
		f := func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://example.com/", nil)
			r.Header.Set("Authorization", tt.header)
			token, ok := BearerAuth(r)
			if ok != tt.ok || token != tt.token {
				t.Errorf("BearerAuth(): got %q, %t, want %q, %t", token, ok, tt.token, tt.ok)
			}
		}
		t.Run(tt.name, f)
	}
}
