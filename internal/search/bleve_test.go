package search

import (
	"context"
	"io"
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/sirupsen/logrus"
)

func TestSearchEmptyQueryReturnsUserTeamDefaults(t *testing.T) {
	searcher := newTestSearcher(t, []slug.Slug{"team-a"}, []Document{
		newTestDocument("team-a", "TEAM", "team-a"),
		newTestDocument("team-b", "TEAM", "team-b"),
		newTestDocument("app-a", "APPLICATION", "app-a", withTeam("team-a")),
		newTestDocument("app-b", "APPLICATION", "app-b", withTeam("team-b")),
	})

	result, err := searcher.Search(newTestContext(), newTestPage(t, 10), SearchFilter{Query: ""})
	if err != nil {
		t.Fatal(err)
	}

	got := testNodeIDs(result.Nodes())
	want := []string{"team-a", "app-a"}
	assertSameElements(t, want, got)
}

func TestSearchEmptyQueryAppliesTeamFilterWithinUserTeams(t *testing.T) {
	searcher := newTestSearcher(t, []slug.Slug{"team-a", "team-b"}, []Document{
		newTestDocument("team-a", "TEAM", "team-a"),
		newTestDocument("team-b", "TEAM", "team-b"),
		newTestDocument("app-a", "APPLICATION", "app-a", withTeam("team-a")),
		newTestDocument("app-b", "APPLICATION", "app-b", withTeam("team-b")),
	})

	result, err := searcher.Search(newTestContext(), newTestPage(t, 10), SearchFilter{
		Query: "",
		Teams: []slug.Slug{"team-b"},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := testNodeIDs(result.Nodes())
	want := []string{"team-b", "app-b"}
	assertSameElements(t, want, got)
}

func TestSearchEmptyQueryWithoutUserTeamsReturnsNoDefaults(t *testing.T) {
	searcher := newTestSearcher(t, nil, []Document{
		newTestDocument("team-a", "TEAM", "team-a"),
		newTestDocument("app-a", "APPLICATION", "app-a", withTeam("team-a")),
	})

	result, err := searcher.Search(newTestContext(), newTestPage(t, 10), SearchFilter{Query: ""})
	if err != nil {
		t.Fatal(err)
	}

	if got := len(result.Nodes()); got != 0 {
		t.Fatalf("expected no default results, got %d", got)
	}
}

type testQuerier struct {
	teamSlugs []slug.Slug
}

func (t testQuerier) TeamSlugsFromUserID(context.Context, uuid.UUID) ([]slug.Slug, error) {
	return t.teamSlugs, nil
}

type testSearchable struct {
	nodes map[string]SearchNode
}

func (t testSearchable) Convert(_ context.Context, ids ...ident.Ident) ([]SearchNode, error) {
	nodes := make([]SearchNode, 0, len(ids))
	for _, id := range ids {
		nodes = append(nodes, t.nodes[id.String()])
	}
	return nodes, nil
}

func (t testSearchable) ReIndex(context.Context) []Document {
	return nil
}

func (t testSearchable) Watch(context.Context, Indexer) error {
	return nil
}

type testNode struct {
	id ident.Ident
}

func (t testNode) ID() ident.Ident {
	return t.id
}

func (t testNode) IsSearchNode() {}

type testUser struct {
	id uuid.UUID
}

func (t testUser) GetID() uuid.UUID {
	return t.id
}

func (t testUser) Identity() string {
	return "test-user"
}

func (t testUser) IsServiceAccount() bool {
	return false
}

func (t testUser) IsAdmin() bool {
	return false
}

func (t testUser) GCPTeamGroups(context.Context) ([]string, error) {
	return nil, nil
}

type documentOption func(*Document)

func withTeam(team string) documentOption {
	return func(doc *Document) {
		doc.Team = team
	}
}

func newTestDocument(id string, kind string, name string, opts ...documentOption) Document {
	docID := ident.Ident{Type: "TEST", ID: id}.String()
	doc := Document{
		ID:   docID,
		Kind: kind,
		Name: name,
		Team: name,
	}
	for _, opt := range opts {
		opt(&doc)
	}
	return doc
}

func newTestSearcher(t *testing.T, teamSlugs []slug.Slug, docs []Document) *bleveSearcher {
	t.Helper()

	indexMapping, err := buildIndexMapping()
	if err != nil {
		t.Fatal(err)
	}

	index, err := bleve.NewMemOnly(indexMapping)
	if err != nil {
		t.Fatal(err)
	}

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	searcher := &bleveSearcher{
		Client:  index,
		Clients: make(map[SearchType]Searchable),
		log:     logger,
		db:      testQuerier{teamSlugs: teamSlugs},
	}

	nodesByKind := map[SearchType]map[string]SearchNode{}
	for _, doc := range docs {
		id := ident.FromString(doc.ID)
		kind := SearchType(doc.Kind)
		if nodesByKind[kind] == nil {
			nodesByKind[kind] = map[string]SearchNode{}
		}
		nodesByKind[kind][doc.ID] = testNode{id: id}
	}

	for kind, nodes := range nodesByKind {
		searcher.AddClient(kind, testSearchable{nodes: nodes})
	}

	if err := searcher.index(docs); err != nil {
		t.Fatal(err)
	}

	return searcher
}

func newTestContext() context.Context {
	return authz.ContextWithActor(context.Background(), testUser{id: uuid.New()}, nil)
}

func newTestPage(t *testing.T, first int) *pagination.Pagination {
	t.Helper()

	page, err := pagination.ParsePage(&first, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

func testNodeIDs(nodes []SearchNode) []string {
	ids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID().ID)
	}
	return ids
}

func assertSameElements(t *testing.T, want []string, got []string) {
	t.Helper()

	if len(want) != len(got) {
		t.Fatalf("expected %v, got %v", want, got)
	}

	wantCounts := make(map[string]int, len(want))
	for _, value := range want {
		wantCounts[value]++
	}

	for _, value := range got {
		wantCounts[value]--
		if wantCounts[value] < 0 {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}
