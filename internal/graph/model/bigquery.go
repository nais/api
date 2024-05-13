package model

import "github.com/nais/api/internal/graph/scalar"

type BigQueryDataset struct {
	CascadingDelete bool         `json:"cascadingDelete"`
	Description     string       `json:"description"`
	Name            string       `json:"name"`
	Permission      string       `json:"permission"`
	ID              scalar.Ident `json:"id"`
}

func (BigQueryDataset) IsPersistence() {}

func (in BigQueryDataset) GetName() string { return in.Name }

func (in BigQueryDataset) GetID() scalar.Ident { return in.ID }
