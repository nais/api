package model

type Role struct {
	Name     string      `json:"name"`
	IsGlobal bool        `json:"isGlobal"`
	GQLVars  RoleGQLVars `json:"-"`
}
