package google_token_source_test

import (
	"testing"

	"github.com/nais/api/internal/thirdparty/google_token_source"
)

func TestNew(t *testing.T) {
	t.Run("empty project ID", func(t *testing.T) {
		builder, err := google_token_source.New("", "domain")
		if builder != nil {
			t.Errorf("expected builder to be nil")
		}

		if err == nil || err.Error() != "empty googleManagementProjectID" {
			t.Errorf("incorrect error returned")
		}
	})

	t.Run("empty domain", func(t *testing.T) {
		builder, err := google_token_source.New("project-id", "")
		if builder != nil {
			t.Errorf("expected builder to be nil")
		}

		if err == nil || err.Error() != "empty domain" {
			t.Errorf("incorrect error returned")
		}
	})
}
