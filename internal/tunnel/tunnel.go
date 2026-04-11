package tunnel

import (
	"time"

	"github.com/nais/api/internal/graph/ident"
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
	TunnelID            string
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
	return ident.Ident{
		ID:   t.TunnelID,
		Type: "Tunnel",
	}
}

func (t *Tunnel) GetName() string                  { return t.Name }
func (t *Tunnel) GetNamespace() string             { return t.TeamSlug }
func (t *Tunnel) GetLabels() map[string]string     { return nil }
func (t *Tunnel) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }
func (t *Tunnel) DeepCopyObject() runtime.Object   { return t }

type CreateTunnelInput struct {
	TeamSlug        string
	EnvironmentName string
	InstanceName    string
	TargetHost      string
	TargetPort      int32
	ClientPublicKey string
}

type CreateTunnelPayload struct {
	Tunnel *Tunnel
}

type UpdateTunnelSTUNEndpointInput struct {
	TunnelID           string
	ClientSTUNEndpoint string
}

type UpdateTunnelSTUNEndpointPayload struct {
	Tunnel *Tunnel
}

type DeleteTunnelInput struct {
	TunnelID string
}

type DeleteTunnelPayload struct {
	Success bool
}

type TeamInventoryCountTunnels struct {
	Total int
}
