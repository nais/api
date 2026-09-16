package valkey

import (
	"context"
	"net/http"
	"testing"

	aivenclient "github.com/aiven/go-client-codegen"
	aivenproject "github.com/aiven/go-client-codegen/handler/project"
	aivenservice "github.com/aiven/go-client-codegen/handler/service"
	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/thirdparty/aiven"
	naiscrd "github.com/nais/pgrator/pkg/api/v1"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValkeyEnvVarSuffix(t *testing.T) {
	tests := []struct {
		name         string
		instanceName string
		want         string
	}{
		{name: "simple name", instanceName: "foo", want: "FOO"},
		{name: "hyphenated name", instanceName: "my-cache", want: "MY_CACHE"},
		{name: "multiple hyphens", instanceName: "my-valkey-instance", want: "MY_VALKEY_INSTANCE"},
		{name: "already uppercase chars", instanceName: "MyCache", want: "_Y_ACHE"},
		{name: "numbers", instanceName: "cache1", want: "CACHE1"},
		{name: "mixed", instanceName: "my-cache-2", want: "MY_CACHE_2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valkeyEnvVarSuffix(tt.instanceName)
			if got != tt.want {
				t.Errorf("valkeyEnvVarSuffix(%q) = %q, want %q", tt.instanceName, got, tt.want)
			}
		})
	}
}

// The fake has Aiven running 9.1, except for names ending in "oldrunning", where it reports 8.1.
func TestUpdateVersionPrecedence(t *testing.T) {
	tests := []struct {
		name      string
		instance  string
		client    aiven.AivenClient
		stored    naiscrd.ValkeyVersion
		requested *ValkeyMajorVersion
		wantSpec  naiscrd.ValkeyVersion
		want      []*ValkeyUpdatedActivityLogEntryDataUpdatedField
	}{
		{
			name:      "request matching the stored version records nothing",
			instance:  "cache",
			client:    aiven.NewFakeAivenClient(),
			stored:    naiscrd.ValkeyVersionV9_1,
			requested: new(ValkeyMajorVersionV9_1),
			wantSpec:  naiscrd.ValkeyVersionV9_1,
		},
		{
			// V9_1 is the only version a client can request for a 9.0 instance.
			name:      "request moves the instance off an end-of-life version",
			instance:  "cache",
			client:    aiven.NewFakeAivenClient(),
			stored:    naiscrd.ValkeyVersionV9_0,
			requested: new(ValkeyMajorVersionV9_1),
			wantSpec:  naiscrd.ValkeyVersionV9_1,
			want: []*ValkeyUpdatedActivityLogEntryDataUpdatedField{
				{Field: "version", OldValue: new("9.0"), NewValue: new("9.1")},
			},
		},
		{
			// Admission refuses the step; the api does not pre-empt it by adopting 9.1 instead.
			name:      "request outranks what Aiven runs",
			instance:  "cache",
			client:    aiven.NewFakeAivenClient(),
			stored:    naiscrd.ValkeyVersionV9_1,
			requested: new(ValkeyMajorVersionV8_1),
			wantSpec:  naiscrd.ValkeyVersionV8_1,
			want: []*ValkeyUpdatedActivityLogEntryDataUpdatedField{
				{Field: "version", OldValue: new("9.1"), NewValue: new("8.1")},
			},
		},
		{
			// Instances created before the api set a version; writing the spec back empty is refused.
			name:     "omitted version takes what Aiven runs",
			instance: "cache",
			client:   aiven.NewFakeAivenClient(),
			wantSpec: naiscrd.ValkeyVersionV9_1,
			want: []*ValkeyUpdatedActivityLogEntryDataUpdatedField{
				{Field: "version", NewValue: new("9.1")},
			},
		},
		{
			name:     "pending upgrade survives an unrelated update",
			instance: "cache-oldrunning",
			client:   aiven.NewFakeAivenClient(),
			stored:   naiscrd.ValkeyVersionV9_1,
			wantSpec: naiscrd.ValkeyVersionV9_1,
		},
		{
			name:     "nothing to fall back to leaves the spec empty",
			instance: "cache",
			client:   instanceMissingAivenClient{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := aiven.NewLoaderContext(context.Background(), aiven.Projects{"dev": {ID: "aiven-dev"}})
			ctx = NewLoaderContext(ctx, "nav", nil, nil, tt.client, logrus.New())

			instance := &naiscrd.Valkey{
				ObjectMeta: metav1.ObjectMeta{Name: tt.instance, Namespace: "myteam"},
				Spec:       naiscrd.ValkeySpec{Version: tt.stored},
			}

			input := UpdateValkeyInput{
				ValkeyInput: ValkeyInput{
					ValkeyMetadataInput: ValkeyMetadataInput{
						Name:            tt.instance,
						EnvironmentName: "dev",
						TeamSlug:        "myteam",
					},
				},
				Version: tt.requested,
			}

			changes, err := updateVersion(ctx, instance, input)
			if err != nil {
				t.Fatalf("updateVersion() error = %v", err)
			}

			if instance.Spec.Version != tt.wantSpec {
				t.Errorf("spec.version = %q, want %q", instance.Spec.Version, tt.wantSpec)
			}
			if diff := cmp.Diff(tt.want, changes); diff != "" {
				t.Errorf("updateVersion() changes (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValkeyMajorVersionMapping(t *testing.T) {
	fromPgrator := []struct {
		in   naiscrd.ValkeyVersion
		want ValkeyMajorVersion
	}{
		{in: naiscrd.ValkeyVersionV8_1, want: ValkeyMajorVersionV8_1},
		{in: naiscrd.ValkeyVersionV9_0, want: ValkeyMajorVersionV9_1},
		{in: naiscrd.ValkeyVersionV9_1, want: ValkeyMajorVersionV9_1},
		{in: "", want: ""},
		{in: "10.0", want: ""},
	}

	for _, tt := range fromPgrator {
		if got := valkeyMajorVersionFromPgrator(tt.in); got != tt.want {
			t.Errorf("valkeyMajorVersionFromPgrator(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	toPgrator := []struct {
		in   ValkeyMajorVersion
		want naiscrd.ValkeyVersion
	}{
		{in: ValkeyMajorVersionV8_1, want: naiscrd.ValkeyVersionV8_1},
		{in: ValkeyMajorVersionV9_1, want: naiscrd.ValkeyVersionV9_1},
		{in: "", want: ""},
		{in: "V9_0", want: ""},
		{in: "V10_0", want: ""},
	}

	for _, tt := range toPgrator {
		if got := tt.in.toPgrator(); got != tt.want {
			t.Errorf("%q.toPgrator() = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// The shared fake answers for every service, so a missing instance needs its own stub.
type instanceMissingAivenClient struct{}

func (instanceMissingAivenClient) ServiceGet(context.Context, string, string, ...[2]string) (*aivenservice.ServiceGetOut, error) {
	return nil, aivenclient.Error{Status: http.StatusNotFound}
}

func (instanceMissingAivenClient) ServiceMaintenanceStart(context.Context, string, string) error {
	return nil
}

func (instanceMissingAivenClient) ProjectAlertsList(context.Context, string) ([]aivenproject.AlertOut, error) {
	return nil, nil
}

func TestGetValkeyVersionSaysNothingWhenAivenLacksInstance(t *testing.T) {
	logger, hook := logrustest.NewNullLogger()

	ctx := aiven.NewLoaderContext(context.Background(), aiven.Projects{"dev": {ID: "aiven-dev"}})
	ctx = NewLoaderContext(ctx, "nav", nil, nil, instanceMissingAivenClient{}, logger)

	instance := &Valkey{
		Name:            "cache",
		TeamSlug:        "myteam",
		EnvironmentName: "dev",
		MajorVersion:    ValkeyMajorVersionV9_1,
	}

	got, err := GetValkeyVersion(ctx, instance)
	if err != nil {
		t.Fatalf("GetValkeyVersion() error = %v", err)
	}

	if got.DesiredMajor != ValkeyMajorVersionV9_1 {
		t.Errorf("desiredMajor = %q, want %q", got.DesiredMajor, ValkeyMajorVersionV9_1)
	}
	if got.Actual != nil {
		t.Errorf("actual = %q, want nil", *got.Actual)
	}
	if entries := hook.AllEntries(); len(entries) != 0 {
		t.Errorf("logged %d entries, want none: %v", len(entries), entries)
	}
}
