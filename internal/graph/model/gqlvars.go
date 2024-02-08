package model

import (
	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
)

type (
	AppGQLVars struct {
		Team slug.Slug
	}

	DeployInfoGQLVars struct {
		App  string
		Job  string
		Env  string
		Team slug.Slug
	}

	InstanceGQLVars struct {
		Env     string
		Team    slug.Slug
		AppName string
	}

	NaisJobGQLVars struct {
		Team slug.Slug
	}

	RoleGQLVars struct {
		TargetServiceAccountID uuid.UUID
		TargetTeamSlug         *slug.Slug
	}

	RunGQLVars struct {
		Env     string
		Team    slug.Slug
		NaisJob string
	}
)
