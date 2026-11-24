package protoapi_test

import (
	"encoding/json"
	"testing"

	"github.com/nais/api/pkg/apiclient/protoapi"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestNaisPostgresDatabaseTypeJSON(t *testing.T) {
	var database protoapi.Database
	if err := protojson.Unmarshal([]byte(`{"type":"NAIS_POSTGRES"}`), &database); err != nil {
		t.Fatalf("unmarshal database type: %v", err)
	}
	if got := database.GetType(); got != protoapi.DatabaseType_NAIS_POSTGRES {
		t.Fatalf("database type = %v, want NAIS_POSTGRES", got)
	}

	data, err := protojson.Marshal(&database)
	if err != nil {
		t.Fatalf("marshal database: %v", err)
	}
	var fields struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("decode database JSON: %v", err)
	}
	if fields.Type != "NAIS_POSTGRES" {
		t.Errorf("serialized database type = %q, want NAIS_POSTGRES", fields.Type)
	}
}
