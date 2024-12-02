package feature

import "github.com/nais/api/internal/graph/ident"

type Features struct {
	Unleash    FeatureUnleash    `json:"unleash"`
	Redis      FeatureRedis      `json:"redis"`
	Kafka      FeatureKafka      `json:"kafka"`
	OpenSearch FeatureOpenSearch `json:"openSearch"`
}

func (f Features) ID() ident.Ident { return NewIdent("container") }
func (f Features) IsNode()         {}

type FeatureUnleash struct {
	Enabled bool `json:"enabled"`
}

func (f FeatureUnleash) ID() ident.Ident { return NewIdent("unleash") }
func (f FeatureUnleash) IsNode()         {}

type FeatureRedis struct {
	Enabled bool `json:"enabled"`
}

func (f FeatureRedis) ID() ident.Ident { return NewIdent("redis") }
func (f FeatureRedis) IsNode()         {}

type FeatureKafka struct {
	Enabled bool `json:"enabled"`
}

func (f FeatureKafka) ID() ident.Ident { return NewIdent("kafka") }
func (f FeatureKafka) IsNode()         {}

type FeatureOpenSearch struct {
	Enabled bool `json:"enabled"`
}

func (f FeatureOpenSearch) ID() ident.Ident { return NewIdent("openSearch") }
func (f FeatureOpenSearch) IsNode()         {}
