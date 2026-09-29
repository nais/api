package protoapi_test

import (
	"encoding/json"
	"testing"

	"github.com/nais/api/pkg/apiclient/protoapi"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestPostgresDatabaseTypes(t *testing.T) {
	if protoapi.DatabaseType_ZALANDO_POSTGRES != 2 || protoapi.DatabaseType_NAIS_POSTGRES != 3 {
		t.Fatal("old and new Postgres database types must retain distinct wire values")
	}

	for _, tt := range []struct {
		name      string
		typeValue protoapi.DatabaseType
	}{
		{name: "ZALANDO_POSTGRES", typeValue: protoapi.DatabaseType_ZALANDO_POSTGRES},
		{name: "NAIS_POSTGRES", typeValue: protoapi.DatabaseType_NAIS_POSTGRES},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var database protoapi.Database
			if err := protojson.Unmarshal([]byte(`{"type":"`+tt.name+`"}`), &database); err != nil {
				t.Fatalf("unmarshal database type %q: %v", tt.name, err)
			}
			if database.GetType() != tt.typeValue {
				t.Fatalf("database type = %v, want %v", database.GetType(), tt.typeValue)
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
			if fields.Type != tt.name {
				t.Errorf("serialized database type = %q, want %q", fields.Type, tt.name)
			}
		})
	}
}
