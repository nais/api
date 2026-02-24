package ident

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/btcsuite/btcutil/base58"
	"github.com/nais/api/internal/graph/model"
)

type Lookup func(ctx context.Context, id Ident) (model.Node, error)

type typeVal struct {
	name   string
	lookup Lookup
}

var knownTypes = map[any]typeVal{}

// RegisterIdentType registers a new ident type with the given key, type name and lookup function.
//
// typeName must be globally unique and should be as short as possible. The lookup function must be able to retrieve the
// node associated with the given ident. The function will panic if lookup is nil, or if the key or the typeName is
// already registered.
//
// This function is typically called during the initialization phase of the packages that defines unique identifiers by
// using the init() function.
func RegisterIdentType[K comparable, T model.Node](key K, typeName string, lookup func(ctx context.Context, id Ident) (T, error)) {
	if lookup == nil {
		panic("lookup function must be set")
	}

	for k, v := range knownTypes {
		if v.name == typeName {
			panic(fmt.Sprintf("ident type already registered for type name: %q", typeName))
		} else if k == key {
			panic("ident type already registered for key")
		}
	}

	knownTypes[key] = typeVal{
		name:   typeName,
		lookup: wrap(lookup),
	}
}

func wrap[T model.Node](fn func(ctx context.Context, id Ident) (T, error)) Lookup {
	return func(ctx context.Context, id Ident) (model.Node, error) {
		return fn(ctx, id)
	}
}

type Ident struct {
	ID   string
	Type string
}

func GetByIdent(ctx context.Context, ident Ident) (model.Node, error) {
	for _, v := range knownTypes {
		if v.name == ident.Type {
			return v.lookup(ctx, ident)
		}
	}
	return nil, fmt.Errorf("unknown ident type")
}

// NewIdent returns a new ident with the given type and id parts. The type must already be registered by using the
// RegisterIdentType function.
func NewIdent[K comparable](t K, id ...string) Ident {
	if _, ok := knownTypes[t]; !ok {
		panic("unknown ident type")
	}

	return Ident{
		ID:   strings.Join(id, "|"),
		Type: knownTypes[t].name,
	}
}

func (i Ident) Parts() []string {
	return strings.Split(i.ID, "|")
}

func (i Ident) MarshalGQLContext(_ context.Context, w io.Writer) error {
	if i.ID == "" || i.Type == "" {
		return fmt.Errorf("id and type must be set")
	}

	_, err := w.Write([]byte(strconv.Quote(i.String())))
	return err
}

func (i Ident) String() string {
	return i.Type + "_" + base58.Encode([]byte(i.ID))
}

func (i *Ident) UnmarshalGQLContext(_ context.Context, v any) error {
	ident, ok := v.(string)
	if !ok {
		return fmt.Errorf("ident must be a string")
	}

	typ, id, ok := strings.Cut(ident, "_")
	if !ok {
		return fmt.Errorf("invalid ident")
	}

	found := false
	for _, v := range knownTypes {
		if v.name == typ {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("unknown ident type")
	}

	i.Type = typ
	i.ID = string(base58.Decode(id))

	return nil
}

func FromString(s string) Ident {
	typ, id, ok := strings.Cut(s, "_")
	if !ok {
		return Ident{}
	}

	return Ident{
		ID:   string(base58.Decode(id)),
		Type: typ,
	}
}
