package tunnel

import (
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Phase string

const (
	PhasePending      Phase = "Pending"
	PhaseProvisioning Phase = "Provisioning"
	PhaseReady        Phase = "Ready"
	PhaseConnected    Phase = "Connected"
	PhaseFailed       Phase = "Failed"
	PhaseTerminated   Phase = "Terminated"
)

type Target struct {
	Host string
	Port int32
}

type Tunnel struct {
	Name                string
	TeamSlug            string
	Environment         string
	Target              Target
	ClientPublicKey     string
	ClientSTUNEndpoint  string
	GatewayPublicKey    string
	GatewaySTUNEndpoint string
	GatewayPodName      string
	Phase               Phase
	Message             string
	CreatedAt           time.Time
}

func (t Tunnel) IsNode() {}

func (t Tunnel) ID() ident.Ident {
	return newTunnelIdent(slug.Slug(t.TeamSlug), t.Environment, t.Name)
}

func (t *Tunnel) GetName() string                  { return t.Name }
func (t *Tunnel) GetNamespace() string             { return t.TeamSlug }
func (t *Tunnel) GetLabels() map[string]string     { return nil }
func (t *Tunnel) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }
func (t *Tunnel) DeepCopyObject() runtime.Object   { return t }

type CreateTunnelInput struct {
	TeamSlug           string
	EnvironmentName    string
	TargetHost         string
	TargetPort         int32
	ClientPublicKey    string
	ClientSTUNEndpoint string
}

type CreateTunnelPayload struct {
	Tunnel *Tunnel
}

type DeleteTunnelInput struct {
	TeamSlug        string
	EnvironmentName string
	TunnelName      string
}

type DeleteTunnelPayload struct {
	Success bool
}

type TeamInventoryCountTunnels struct {
	Total int
}
