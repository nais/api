package ident

import (
	"context"
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/graph/model"
)

type (
	keyType1 int
	keyType2 int
)

type TestModel struct {
	Ident Ident
	Type  string
}

func (TestModel) IsNode() {}

func ensureEmptyKnownTypes() func() {
	// Ensure empty knownTypes when running test
	old := maps.Clone(knownTypes)
	knownTypes = make(map[any]typeVal)
	return func() {
		knownTypes = old
	}
}

func TestRegisterType(t *testing.T) {
	defer ensureEmptyKnownTypes()()

	// Register a few that should not panic
	RegisterIdentType(keyType1(1), "type1", func(ctx context.Context, id Ident) (model.Node, error) { return nil, nil })
	RegisterIdentType(keyType2(1), "type2", func(ctx context.Context, id Ident) (model.Node, error) { return nil, nil })

	// Register the same type name again
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()
		RegisterIdentType(keyType1(3), "type1", func(ctx context.Context, id Ident) (model.Node, error) { return nil, nil })
	}()

	// Register the same key again
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()
		RegisterIdentType(keyType1(1), "type3", func(ctx context.Context, id Ident) (model.Node, error) { return nil, nil })
	}()
}

func TestGetByIdent(t *testing.T) {
	defer ensureEmptyKnownTypes()()

	RegisterIdentType(keyType1(1), "type1", func(ctx context.Context, id Ident) (*TestModel, error) {
		return &TestModel{Ident: id, Type: "type1"}, nil
	})
	RegisterIdentType(keyType2(2), "type2", func(ctx context.Context, id Ident) (*TestModel, error) {
		return &TestModel{Ident: id, Type: "type2"}, nil
	})

	o, err := GetByIdent(context.Background(), NewIdent(keyType1(1), "id1"))
	if err != nil {
		t.Error(err)
	}
	if o.(*TestModel).Type != "type1" {
		t.Error("unexpected type")
	}

	// GetByIdent unknown key
	unknownIdent := Ident{Type: "unknown", ID: "id2"}
	_, err = GetByIdent(context.Background(), unknownIdent)
	if err == nil {
		t.Error("expected error")
	}
}

func TestNewIdent(t *testing.T) {
	defer ensureEmptyKnownTypes()()

	RegisterIdentType(keyType1(1), "type1", func(ctx context.Context, id Ident) (*TestModel, error) {
		return &TestModel{Ident: id, Type: "type1"}, nil
	})
	RegisterIdentType(keyType2(2), "type2", func(ctx context.Context, id Ident) (*TestModel, error) {
		return &TestModel{Ident: id, Type: "type2"}, nil
	})

	o := NewIdent(keyType1(1), "idpart1", "idpart2")
	if o.Type != "type1" {
		t.Error("unexpected type")
	}

	if expectedID := "idpart1|idpart2"; o.ID != expectedID {
		t.Errorf("expected id %q, got %q", expectedID, o.ID)
	}

	if diff := cmp.Diff([]string{"idpart1", "idpart2"}, o.Parts()); diff != "" {
		t.Errorf("diff: -want +got\n%s", diff)
	}

	// NewIdent unknown key
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()
		_ = NewIdent(keyType1(2), "id2")
	}()
}
