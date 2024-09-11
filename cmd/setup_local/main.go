package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/nais/api/internal/vulnerabilities"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"
	"unicode"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/logger"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/usersync"
	"github.com/sethvargo/go-envconfig"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/ptr"
)

type seedConfig struct {
	DatabaseURL               string `env:"DATABASE_URL,default=postgres://api:api@localhost:3002/api?sslmode=disable"`
	Domain                    string `env:"TENANT_DOMAIN,default=example.com"`
	GoogleManagementProjectID string `env:"GOOGLE_MANAGEMENT_PROJECT_ID"`

	NumUsers          *int
	NumTeams          *int
	NumOwnersPerTeam  *int
	NumMembersPerTeam *int
	DependencyTrack   *DependencyTrack
	ForceSeed         *bool
	ProvisionPubSub   *bool
}

type DependencyTrack struct {
	Url      string `env:"DEPENDENCY_TRACK_URL,default=http://localhost:9010"`
	Username string `env:"DEPENDENCY_TRACK_USERNAME,default=admin"`
	Password string `env:"DEPENDENCY_TRACK_PASSWORD,default=yolo"`
}

func newSeedConfig(ctx context.Context) (*seedConfig, error) {
	cfg := &seedConfig{}
	err := envconfig.Process(ctx, cfg)
	if err != nil {
		return nil, err
	}

	cfg.NumUsers = flag.Int("users", 1000, "number of users to insert")
	cfg.NumTeams = flag.Int("teams", 200, "number of teams to insert")
	cfg.NumOwnersPerTeam = flag.Int("owners", 3, "number of owners per team")
	cfg.NumMembersPerTeam = flag.Int("members", 10, "number of members per team")
	cfg.ForceSeed = flag.Bool("force", false, "seed regardless of existing database content")
	cfg.ProvisionPubSub = flag.Bool("provision_pub_sub", true, "set up pubsub credentials")
	cfg.DependencyTrack.Username = *flag.String("dependency_track_username", "admin", "username for dependency track")
	cfg.DependencyTrack.Password = *flag.String("dependency_track_password", "yolo", "password for dependency track")
	cfg.DependencyTrack.Url = *flag.String("dependency_track_url", "http://localhost:9010", "url for dependency track")
	flag.Parse()

	return cfg, nil
}

func main() {
	ctx := context.Background()
	cfg, err := newSeedConfig(ctx)
	if err != nil {
		fmt.Printf("fatal: %s", err)
		os.Exit(1)
	}

	log, err := logger.New("text", "INFO")
	if err != nil {
		fmt.Printf("fatal: %s", err)
		os.Exit(2)
	}

	err = run(ctx, cfg, log)
	if err != nil {
		log.WithError(err).Error("fatal error in run()")
		os.Exit(3)
	}
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

		log.Infof("creating subscription")

		if _, err := client.CreateSubscription(ctx, "api-reconcilers-api-events", pubsub.SubscriptionConfig{
			Topic:             client.Topic("nais-api"),
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

	db, close, err := database.New(ctx, cfg.DatabaseURL, log)
	if err != nil {
		return err
	}
	defer close()

	emails := map[string]struct{}{}
	slugs := map[string]struct{}{}

	uploadFakeSbom(cfg.DependencyTrack, log)

	if !*cfg.ForceSeed {
		if existingUsers, err := getAllUsers(ctx, db); len(existingUsers) != 0 || err != nil {
			return fmt.Errorf("database already has users, abort: %w", err)
		}

		if existingTeams, err := getAllTeams(ctx, db); len(existingTeams) != 0 || err != nil {
			return fmt.Errorf("database already has teams, abort: %w", err)
		}
	} else {
		users, err := getAllUsers(ctx, db)
		if err != nil {
			return err
		}
		for _, user := range users {
			emails[user.Email] = struct{}{}
		}

		teams, err := getAllTeams(ctx, db)
		if err != nil {
			return err
		}
		for _, team := range teams {
			slugs[string(team.Slug)] = struct{}{}
		}
	}

	err = db.Transaction(ctx, func(ctx context.Context, dbtx database.Database) error {
		var err error
		var devUser, adminUser *database.User

		devUser, err = dbtx.GetUserByEmail(ctx, nameToEmail("dev usersen", cfg.Domain))
		if err != nil {
			devUser, err = dbtx.CreateUser(ctx, "dev usersen", nameToEmail("dev usersen", cfg.Domain), uuid.New().String())
			if err != nil {
				return err
			}
		}

		adminUser, err = dbtx.GetUserByEmail(ctx, nameToEmail("admin usersen", cfg.Domain))
		if err != nil {
			adminUser, err = dbtx.CreateUser(ctx, "admin usersen", nameToEmail("admin usersen", cfg.Domain), uuid.New().String())
			if err != nil {
				return err
			}
		}

		if err != nil {
			return err
		}
		if err = dbtx.AssignGlobalRoleToUser(ctx, adminUser.ID, gensql.RoleNameAdmin); err != nil {
			return err
		}
		for _, roleName := range usersync.DefaultRoleNames {
			err = dbtx.AssignGlobalRoleToUser(ctx, devUser.ID, roleName)
			if err != nil {
				return fmt.Errorf("attach default role %q to user %q: %w", roleName, devUser.Email, err)
			}
		}
		users := []*database.User{devUser}
		for i := 1; i <= *cfg.NumUsers; i++ {
			firstName := firstNames[rand.Intn(numFirstNames)]
			lastName := lastNames[rand.Intn(numLastNames)]
			name := firstName + " " + lastName
			email := nameToEmail(name, cfg.Domain)
			if _, exists := emails[email]; exists {
				continue
			}

			user, err := dbtx.CreateUser(ctx, name, email, uuid.New().String())
			if err != nil {
				return err
			}

			for _, roleName := range usersync.DefaultRoleNames {
				err = dbtx.AssignGlobalRoleToUser(ctx, user.ID, roleName)
				if err != nil {
					return fmt.Errorf("attach default role %q to user %q: %w", roleName, user.Email, err)
				}
			}

			log.Infof("%d/%d users created", i, *cfg.NumUsers)
			users = append(users, user)
			emails[email] = struct{}{}
		}
		usersCreated := len(users)

		var devteam *database.Team
		devteam, err = dbtx.GetTeamBySlug(ctx, "devteam")
		if err != nil {
			devteam, err = dbtx.CreateTeam(ctx, "devteam", "dev-purpose", "#devteam")
			if err != nil {
				return err
			}
		}
		if _, err := dbtx.UpdateTeamExternalReferences(ctx, gensql.UpdateTeamExternalReferencesParams{
			GoogleGroupEmail: ptr.To("nais-team-devteam@" + cfg.Domain),
			AzureGroupID:     ptr.To(uuid.MustParse("70c0541d-c079-4d10-9c50-164681e0b900")),
			GithubTeamSlug:   ptr.To("devteam"),
			GarRepository:    ptr.To("projects/some-project-123/locations/europe-north1/repositories/devteam"),
			Slug:             devteam.Slug,
		}); err != nil {
			return err
		}

		err = dbtx.SetTeamMemberRole(ctx, devUser.ID, devteam.Slug, gensql.RoleNameTeamowner)
		if err != nil {
			return err
		}

		if err = dbtx.UpsertTeamEnvironment(ctx, devteam.Slug, "dev", ptr.To("#yolo"), ptr.To("nais-dev-2e7b")); err != nil {
			return err
		}

		if err = dbtx.UpsertTeamEnvironment(ctx, devteam.Slug, "superprod", ptr.To("#yolo"), ptr.To("nais-dev-cdea")); err != nil {
			return err
		}

		for i := 1; i <= *cfg.NumTeams; i++ {
			name := teamName()
			if _, exists := slugs[name]; exists {
				continue
			}

			team, err := dbtx.CreateTeam(ctx, slug.Slug(name), "some purpose", "#"+name)
			if err != nil {
				return err
			}

			for o := 0; o < *cfg.NumOwnersPerTeam; o++ {
				err = dbtx.SetTeamMemberRole(ctx, users[rand.Intn(usersCreated)].ID, team.Slug, gensql.RoleNameTeamowner)
				if err != nil {
					return err
				}
			}

			for o := 0; o < *cfg.NumMembersPerTeam; o++ {
				err = dbtx.SetTeamMemberRole(ctx, users[rand.Intn(usersCreated)].ID, team.Slug, gensql.RoleNameTeammember)
				if err != nil {
					return err
				}
			}

			log.Infof("%d/%d teams created", i, *cfg.NumTeams)
			slugs[name] = struct{}{}
		}

		return nil
	})
	if err != nil {
		return err
	}

	log.Infof("done")
	return nil
}

func teamName() string {
	letters := []byte("abcdefghijklmnopqrstuvwxyz")
	b := make([]byte, 10)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func nameToEmail(name, domain string) string {
	name = strings.NewReplacer(" ", ".", "æ", "ae", "ø", "oe", "å", "aa").Replace(strings.ToLower(name))
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	name, _, _ = transform.String(t, name)
	return name + "@" + domain
}

func fileToSlice(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, nil
}

func getAllUsers(ctx context.Context, db database.UserRepo) ([]*database.User, error) {
	limit, offset := 100, 0
	users := make([]*database.User, 0)
	for {
		page, _, err := db.GetUsers(ctx, database.Page{
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}

		users = append(users, page...)

		if len(page) < limit {
			break
		}

		offset += limit
	}

	return users, nil
}

func getAllTeams(ctx context.Context, db database.TeamRepo) ([]*database.Team, error) {
	limit, offset := 100, 0
	teams := make([]*database.Team, 0)
	for {
		page, _, err := db.GetTeams(ctx, database.Page{
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}

		teams = append(teams, page...)

		if len(page) < limit {
			break
		}

		offset += limit
	}

	return teams, nil
}

func uploadFakeSbom(track *DependencyTrack, log logrus.FieldLogger) {
	ctx := context.Background()
	client := vulnerabilities.NewManager(
		&vulnerabilities.Config{
			DependencyTrack: vulnerabilities.DependencyTrackConfig{
				Endpoint:    track.Url,
				Username:    track.Username,
				Password:    track.Password,
				FrontendUrl: "http://localhost:9020",
			},
		},
	)
	sbom, err := getSbom()
	err = client.UploadProject(ctx, "ghcr.io/nais/testapp/testapp", "nais-deploy-canary", "2020-02-25-f61e7b7", "devteam", sbom)
	if err != nil {
		log.Fatalf("upload project: %v", err)
	}
}

func getSbom() ([]byte, error) {
	read, err := os.ReadFile("data/sbom/sbom.json")
	if err != nil {
		log.Fatalf("read file: %v", err)
	}

	metadata := &in_toto.CycloneDXStatement{}
	err = json.Unmarshal(read, metadata)
	if err != nil {
		log.Fatalf("unmarshal: %v", err)
	}

	sbom, err := json.Marshal(metadata.Predicate)
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}
	return sbom, nil
}
