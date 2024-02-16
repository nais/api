package authn

import (
	"net/url"
	"testing"
)

func TestRedirectURI(t *testing.T) {
	baseUrl, err := url.Parse("https://teams.test/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("test values", func(t *testing.T) {
		tests := []struct {
			raw   string
			path  string
			query string
		}{
			{
				raw:   "/teams?selection=my",
				path:  "/teams",
				query: "selection=my",
			},
			{
				raw:   "/teams",
				path:  "/teams",
				query: "",
			},
			{
				raw:   "%2Fteams%3Fselection%3Dmy",
				path:  "/teams",
				query: "selection=my",
			},
		}
		for _, tt := range tests {
			baseUrlCopy := baseUrl
			updateRedirectURL(baseUrlCopy, tt.raw)
			if baseUrlCopy.Path != tt.path {
				t.Errorf("expected %q, got %q", tt.path, baseUrlCopy.Path)
			}

			if baseUrlCopy.RawQuery != tt.query {
				t.Errorf("expected %q, got %q", tt.query, baseUrlCopy.RawQuery)
			}
		}
	})
}
