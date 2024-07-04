package scalar

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/nais/api/internal/graphv1/modelv1"
)

type Lookup func(ctx context.Context, id Ident) (modelv1.Node, error)

type typeVal struct {
	name   string
	lookup Lookup
}

var knownTypes = map[any]typeVal{}

func Wrap[T modelv1.Node](fn func(ctx context.Context, id Ident) (T, error)) Lookup {
	return func(ctx context.Context, id Ident) (modelv1.Node, error) {
		return fn(ctx, id)
	}
}

func RegisterIdentType[K comparable](key K, t string, lookup Lookup) {
	if lookup == nil {
		panic("lookup function must be set")
	}

	for k, v := range knownTypes {
		if v.name == t {
			panic("ident type already registered for type " + t)
		} else if k == key {
			panic("ident type already registered for key")
		}
	}

	knownTypes[key] = typeVal{
		name:   t,
		lookup: lookup,
	}
}

type Ident struct {
	ID   string
	Type string
}

func GetByIdent(ctx context.Context, ident Ident) (modelv1.Node, error) {
	for _, v := range knownTypes {
		if v.name == ident.Type {
			return v.lookup(ctx, ident)
		}
	}
	return nil, fmt.Errorf("unknown ident type")
}

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
	v := url.Values{}
	v.Set("id", i.ID)
	v.Set("type", string(i.Type))
	_, err := w.Write([]byte(strconv.Quote(base64.URLEncoding.EncodeToString([]byte(v.Encode())))))
	return err
}

func (i *Ident) UnmarshalGQLContext(_ context.Context, v interface{}) error {
	ident, ok := v.(string)
	if !ok {
		return fmt.Errorf("ident must be a string")
	}

	bytes, err := base64.URLEncoding.DecodeString(ident)
	if err != nil {
		return err
	}

	values, err := url.ParseQuery(string(bytes))
	if err != nil {
		return err
	}

	i.ID = values.Get("id")
	i.Type = values.Get("type")

	return nil
}
