package authn

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedirectURI(t *testing.T) {
	baseUrl, err := url.Parse("https://teams.test/")
	assert.NoError(t, err)

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
			assert.Equal(t, baseUrlCopy.Path, tt.path)
			assert.Equal(t, baseUrlCopy.RawQuery, tt.query)
		}
	})
}
