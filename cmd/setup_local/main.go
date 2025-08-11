package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"time"
	"unicode"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/logger"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/user"
	"github.com/nais/api/internal/usersync/usersyncer"
	"github.com/nais/api/internal/usersync/usersyncsql"
	"github.com/sethvargo/go-envconfig"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/ptr"
)

const (
	exitCodeSuccess = iota
	exitCodeConfigError
	exitCodeLoggerError
	exitCodeRunError
)

type seedConfig struct {
	DatabaseURL               string `env:"DATABASE_URL,default=postgres://api:api@localhost:3002/api?sslmode=disable"`
	Domain                    string `env:"TENANT_DOMAIN,default=example.com"`
	GoogleManagementProjectID string `env:"GOOGLE_MANAGEMENT_PROJECT_ID,default=nais-local-dev"`

	NumUsers          *int
	NumTeams          *int
	NumOwnersPerTeam  *int
	NumMembersPerTeam *int
	ForceSeed         *bool
	ProvisionPubSub   *bool
}

func newSeedConfig(ctx context.Context) (*seedConfig, error) {
	cfg := &seedConfig{}
	if err := envconfig.Process(ctx, cfg); err != nil {
		return nil, err
	}

	cfg.NumUsers = flag.Int("users", 1000, "number of users to insert")
	cfg.NumTeams = flag.Int("teams", 200, "number of teams to insert")
	cfg.NumOwnersPerTeam = flag.Int("owners", 3, "number of owners per team")
	cfg.NumMembersPerTeam = flag.Int("members", 10, "number of members per team")
	cfg.ForceSeed = flag.Bool("force", false, "seed regardless of existing database content")
	cfg.ProvisionPubSub = flag.Bool("provision_pub_sub", true, "set up pubsub credentials")
	flag.Parse()

	return cfg, nil
}

func main() {
	ctx := context.Background()
	log, err := logger.New("text", "INFO")
	if err != nil {
		fmt.Printf("log error: %s", err)
		os.Exit(exitCodeLoggerError)
	}

	cfg, err := newSeedConfig(ctx)
	if err != nil {
		log.WithError(err).Errorf("configuration error")
		os.Exit(exitCodeConfigError)
	}

	if err := run(ctx, cfg, log); err != nil {
		log.WithError(err).Errorf("fatal error in run()")
		os.Exit(exitCodeRunError)
	}

	os.Exit(exitCodeSuccess)
}

func run(ctx context.Context, cfg *seedConfig, log logrus.FieldLogger) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if *cfg.ProvisionPubSub {
		log.Infof("Provisioning pubsub")

		if err := os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:3004"); err != nil {
			return err
		}

		client, err := pubsub.NewClient(ctx, cfg.GoogleManagementProjectID)
		if err != nil {
			return err
		}

		log.Infof("creating topic")

		if _, err := client.CreateTopic(ctx, "nais-api"); err != nil {
			if s, ok := status.FromError(err); !ok || s.Code() != codes.AlreadyExists {
				return err
			}
		}

		client.Topic("nais-api-log-topic").Delete(ctx)
		if _, err := client.CreateTopic(ctx, "nais-api-log-topic"); err != nil {
			if s, ok := status.FromError(err); !ok || s.Code() != codes.AlreadyExists {
				return err
			}
		}

		log.Infof("creating subscription")

		if _, err := client.CreateSubscription(ctx, "nais-api-reconcilers-api-events", pubsub.SubscriptionConfig{
			Topic:             client.Topic("nais-api"),
			RetentionDuration: 1 * time.Hour,
		}); err != nil {
			if s, ok := status.FromError(err); !ok || s.Code() != codes.AlreadyExists {
				return err
			}
		}
		if _, err := client.CreateSubscription(ctx, "nais-api-log-topic-subscription", pubsub.SubscriptionConfig{
			Topic:             client.Topic("nais-api-log-topic"),
			RetentionDuration: 1 * time.Hour,
		}); err != nil {
			if s, ok := status.FromError(err); !ok || s.Code() != codes.AlreadyExists {
				return err
			}
		}
	}

	firstNames, err := fileToSlice("data/first_names.txt")
	if err != nil {
		return err
	}
	numFirstNames := len(firstNames)

	lastNames, err := fileToSlice("data/last_names.txt")
	if err != nil {
		return err
	}
	numLastNames := len(lastNames)

	log.Infof("initializing database")

	pool, err := database.New(ctx, cfg.DatabaseURL, log)
	if err != nil {
		return err
	}
	defer pool.Close()

	ctx = database.NewLoaderContext(ctx, pool)
	ctx = activitylog.NewLoaderContext(ctx, pool)
	ctx = user.NewLoaderContext(ctx, pool)
	ctx = team.NewLoaderContext(ctx, pool, nil)
	ctx = authz.NewLoaderContext(ctx, pool)
	ctx = environment.NewLoaderContext(ctx, pool)

	emails := map[string]struct{}{}
	slugs := map[slug.Slug]struct{}{}

	if !*cfg.ForceSeed {
		if existingUsers, err := getAllUsers(ctx); err != nil {
			return fmt.Errorf("fetch existing users: %w", err)
		} else if len(existingUsers) != 0 {
			return fmt.Errorf("database already has users, abort")
		}

		if existingTeams, err := getAllTeams(ctx); err != nil {
			return fmt.Errorf("fetch existing teams: %w", err)
		} else if len(existingTeams) != 0 {
			return fmt.Errorf("database already has teams, abort")
		}
	} else {
		users, err := getAllUsers(ctx)
		if err != nil {
			return fmt.Errorf("fetch existing users: %w", err)
		}
		for _, u := range users {
			emails[u.Email] = struct{}{}
		}

		teams, err := getAllTeams(ctx)
		if err != nil {
			return fmt.Errorf("fetch existing teams: %w", err)
		}
		for _, t := range teams {
			slugs[t.Slug] = struct{}{}
		}
	}

	err = database.Transaction(ctx, func(ctx context.Context) error {
		const (
			adminName = "admin usersen"
			devName   = "dev usersen"

			devEnvironment  = "dev"
			prodEnvironment = "superprod"
			kindEnvironment = "kind-kind"
		)

		envs := []*environment.Environment{
			{
				Name: kindEnvironment,
				GCP:  false,
			},
			{
				Name: devEnvironment,
				GCP:  true,
			},
			{
				Name: prodEnvironment,
				GCP:  true,
			},
		}
		if err := environment.SyncEnvironments(ctx, envs); err != nil {
			return fmt.Errorf("sync environments: %w", err)
		}

		var err error
		var adminUser, devUser *user.User

		usersyncq := usersyncsql.New(database.TransactionFromContext(ctx))

		createUser := func(ctx context.Context, name, email string) (*user.User, error) {
			usu, err := usersyncq.Create(ctx, usersyncsql.CreateParams{
				Name:       name,
				Email:      email,
				ExternalID: uuid.New().String(),
			})
			if err != nil {
				return nil, fmt.Errorf("create user: %w", err)
			}

			usr, err := user.GetByEmail(ctx, usu.Email)
			if err != nil {
				return nil, fmt.Errorf("get user: %w", err)
			}

			return usr, nil
		}

		adminUser, err = user.GetByEmail(ctx, nameToEmail(adminName, cfg.Domain))
		if err != nil {
			adminUser, err = createUser(ctx, adminName, nameToEmail(adminName, cfg.Domain))
			if err != nil {
				return fmt.Errorf("create admin user: %w", err)
			}
		}

		if err := usersyncq.AssignGlobalAdmin(ctx, adminUser.UUID); err != nil {
			return fmt.Errorf("assign global admin role to admin user: %w", err)
		}
		actor := &authz.Actor{User: adminUser}

		devUser, err = user.GetByEmail(ctx, nameToEmail(devName, cfg.Domain))
		if err != nil {
			devUser, err = createUser(ctx, devName, nameToEmail(devName, cfg.Domain))
			if err != nil {
				return fmt.Errorf("create dev user: %w", err)
			}
		}

		if err := usersyncer.AssignDefaultPermissionsToUser(ctx, usersyncq, devUser.UUID); err != nil {
			return fmt.Errorf("assign default permissions to dev user: %w", err)
		}

		users := []*user.User{devUser}
		for i := 1; i <= *cfg.NumUsers; i++ {
			firstName := firstNames[rand.IntN(numFirstNames)]
			lastName := lastNames[rand.IntN(numLastNames)]
			name := firstName + " " + lastName
			email := nameToEmail(name, cfg.Domain)
			if _, exists := emails[email]; exists {
				continue
			}

			u, err := createUser(ctx, name, email)
			if err != nil {
				return fmt.Errorf("create user %q: %w", email, err)
			}

			if err = usersyncer.AssignDefaultPermissionsToUser(ctx, usersyncq, u.UUID); err != nil {
				return fmt.Errorf("assign default permissions to user %q: %w", u.Email, err)
			}

			log.Infof("%d/%d users created", i, *cfg.NumUsers)
			users = append(users, u)
			emails[email] = struct{}{}
		}

		var devteam *team.Team
		devteam, err = team.Get(ctx, "devteam")
		if err != nil {
			input := &team.CreateTeamInput{
				Slug:         "devteam",
				Purpose:      "dev-purpose",
				SlackChannel: "#devteam",
			}
			devteam, err = team.Create(ctx, input, actor)
			if err != nil {
				return fmt.Errorf("create devteam: %w", err)
			}
		}

		references := &team.ExternalReferences{
			GoogleGroupEmail: ptr.To("nais-team-devteam@" + cfg.Domain),
			EntraIDGroupID:   ptr.To(uuid.MustParse("70c0541d-c079-4d10-9c50-164681e0b900")),
			GithubTeamSlug:   ptr.To("devteam"),
			GarRepository:    ptr.To("projects/some-project-123/locations/europe-north1/repositories/devteam"),
		}
		if err := team.UpdateExternalReferences(ctx, devteam.Slug, references); err != nil {
			return fmt.Errorf("update external references for devteam: %w", err)
		}

		if err := authz.MakeUserTeamOwner(ctx, devUser.UUID, devteam.Slug); err != nil {
			return fmt.Errorf("make user %q owner of team %q: %w", devUser.Email, devteam.Slug, err)
		}

		input := &team.UpdateTeamEnvironmentInput{
			Slug:               devteam.Slug,
			EnvironmentName:    kindEnvironment,
			SlackAlertsChannel: ptr.To("#kind"),
			GCPProjectID:       ptr.To("kind-kind"),
		}
		if _, err := team.UpdateEnvironment(ctx, input, actor); err != nil {
			return fmt.Errorf("update environment %q for devteam: %w", kindEnvironment, err)
		}

		input = &team.UpdateTeamEnvironmentInput{
			Slug:               devteam.Slug,
			EnvironmentName:    devEnvironment,
			SlackAlertsChannel: ptr.To("#yolo"),
			GCPProjectID:       ptr.To("nais-dev-2e7b"),
		}
		if _, err := team.UpdateEnvironment(ctx, input, actor); err != nil {
			return fmt.Errorf("update environment %q for devteam: %w", devEnvironment, err)
		}

		input = &team.UpdateTeamEnvironmentInput{
			Slug:               devteam.Slug,
			EnvironmentName:    prodEnvironment,
			SlackAlertsChannel: ptr.To("#yolo"),
			GCPProjectID:       ptr.To("nais-dev-cdea"),
		}
		if _, err := team.UpdateEnvironment(ctx, input, actor); err != nil {
			return fmt.Errorf("update environment %q for devteam: %w", prodEnvironment, err)
		}

		for i := 1; i <= *cfg.NumTeams; i++ {
			name := teamName()
			if _, exists := slugs[name]; exists {
				continue
			}

			input := &team.CreateTeamInput{
				Slug:         name,
				Purpose:      "some purpose",
				SlackChannel: "#" + name.String(),
			}
			t, err := team.Create(ctx, input, actor)
			if err != nil {
				return fmt.Errorf("create team %q: %w", name, err)
			}

			for o := 0; o < *cfg.NumOwnersPerTeam; o++ {
				u := users[rand.IntN(len(users))]
				if err = authz.MakeUserTeamOwner(ctx, u.UUID, t.Slug); err != nil {
					return fmt.Errorf("make user %q owner of team %q: %w", u.Email, t.Slug, err)
				}
			}

			for o := 0; o < *cfg.NumMembersPerTeam; o++ {
				u := users[rand.IntN(len(users))]
				if err = authz.MakeUserTeamMember(ctx, u.UUID, t.Slug); err != nil {
					return fmt.Errorf("make user %q member of team %q: %w", u.Email, t.Slug, err)
				}
			}

			log.Infof("%d/%d teams created", i, *cfg.NumTeams)
			slugs[name] = struct{}{}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("error during transaction: %w", err)
	}

	log.Infof("done")
	return nil
}

func teamName() slug.Slug {
	letters := []byte("abcdefghijklmnopqrstuvwxyz")
	b := make([]byte, 10)
	for i := range b {
		b[i] = letters[rand.IntN(len(letters))]
	}
	return slug.Slug(b)
}

func nameToEmail(name, domain string) string {
	name = strings.NewReplacer(" ", ".", "æ", "ae", "ø", "oe", "å", "aa").Replace(strings.ToLower(name))
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	name, _, _ = transform.String(t, name)
	return name + "@" + domain
}

func fileToSlice(path string) ([]string, error) {
	file, err := os.Open(path) // #nosec: G304
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, nil
}

func getAllUsers(ctx context.Context) ([]*user.User, error) {
	first := 100
	allUsers := make([]*user.User, 0)
	orderBy := &user.UserOrder{
		Field:     user.UserOrderFieldName,
		Direction: model.OrderDirectionAsc,
	}
	var after *pagination.Cursor
	for {
		p, err := pagination.ParsePage(&first, after, nil, nil)
		if err != nil {
			return nil, err
		}
		conn, err := user.List(ctx, p, orderBy)
		if err != nil {
			return nil, err
		}
		allUsers = append(allUsers, conn.Nodes()...)
		if !conn.PageInfo.HasNextPage {
			break
		}
		after = conn.PageInfo.EndCursor
	}

	return allUsers, nil
}

func getAllTeams(ctx context.Context) ([]*team.Team, error) {
	first := 100
	allTeams := make([]*team.Team, 0)
	orderBy := &team.TeamOrder{
		Field:     team.TeamOrderFieldSlug,
		Direction: model.OrderDirectionAsc,
	}
	var after *pagination.Cursor
	for {
		p, err := pagination.ParsePage(&first, after, nil, nil)
		if err != nil {
			return nil, err
		}
		conn, err := team.List(ctx, p, orderBy, nil)
		if err != nil {
			return nil, err
		}
		allTeams = append(allTeams, conn.Nodes()...)
		if !conn.PageInfo.HasNextPage {
			break
		}
		after = conn.PageInfo.EndCursor
	}

	return allTeams, nil
}
