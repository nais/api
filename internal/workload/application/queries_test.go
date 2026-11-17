package application

import (
	"context"
	"encoding/json"
	"testing"
	"testing/fstest"
	"time"

	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestGetIngressTypeByHostPreservesWorkloadScope(t *testing.T) {
	fixtures := fstest.MapFS{}
	for _, tt := range []struct {
		cluster, team, app, name, class, host string
	}{
		{"dev", "team", "app", "internal", "nais-ingress", "shared.example.com"},
		{"prod", "team", "app", "external", "nais-ingress-external", "shared.example.com"},
		{"dev", "other-team", "app", "authenticated", "nais-ingress-fa", "shared.example.com"},
		{"dev", "team", "other-app", "unknown", "unknown", "shared.example.com"},
		{"dev", "team", "app", "second-host", "internal-haproxy", "second.example.com"},
	} {
		ingress := &netv1.Ingress{
			TypeMeta: metav1.TypeMeta{APIVersion: "networking.k8s.io/v1", Kind: "Ingress"},
			ObjectMeta: metav1.ObjectMeta{
				Name: tt.name, Namespace: tt.team, Labels: map[string]string{"app": tt.app},
			},
			Spec: netv1.IngressSpec{
				IngressClassName: ptr.To(tt.class),
				Rules:            []netv1.IngressRule{{Host: tt.host}},
			},
		}
		data, err := json.Marshal(ingress)
		if err != nil {
			t.Fatal(err)
		}
		fixtures[tt.cluster+"/"+tt.team+"/"+tt.name+".yaml"] = &fstest.MapFile{Data: data}
	}
	log, _ := logrustest.NewNullLogger()
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := watcher.NewManager(scheme, kubernetes.ClusterConfigMap{"dev": nil, "prod": nil}, log, watcher.WithClientCreator(fake.Clients(fixtures)))
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Stop()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ingressWatcher := NewIngressWatcher(ctx, mgr)
	waitCtx, waitCancel := context.WithTimeout(ctx, 5*time.Second)
	defer waitCancel()
	if !mgr.WaitForReady(waitCtx) {
		t.Fatal("timed out waiting for ingress watcher")
	}
	ctx = NewLoaderContext(ctx, nil, ingressWatcher, log)

	for _, tt := range []struct {
		name, cluster, team, app, url string
		want                          IngressType
	}{
		{"internal", "dev", "team", "app", "https://shared.example.com", IngressTypeInternal},
		{"other environment", "prod", "team", "app", "https://shared.example.com", IngressTypeExternal},
		{"other team", "dev", "other-team", "app", "https://shared.example.com", IngressTypeAuthenticated},
		{"other application", "dev", "team", "other-app", "https://shared.example.com", IngressTypeUnknown},
		{"second host", "dev", "team", "app", "https://second.example.com/path", IngressTypeInternal},
		{"missing environment", "missing", "team", "app", "https://shared.example.com", IngressTypeUnknown},
		{"missing team", "dev", "missing", "app", "https://shared.example.com", IngressTypeUnknown},
		{"missing application", "dev", "team", "missing", "https://shared.example.com", IngressTypeUnknown},
		{"missing host", "dev", "team", "app", "https://missing.example.com", IngressTypeUnknown},
		{"exact host matching", "dev", "team", "app", "https://SHARED.EXAMPLE.COM", IngressTypeUnknown},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := GetIngressType(ctx, &Ingress{
				URL: tt.url, EnvironmentName: tt.cluster, TeamSlug: slug.Slug(tt.team), ApplicationName: tt.app,
			})
			if got != tt.want {
				t.Errorf("GetIngressType() = %s, want %s", got, tt.want)
			}
		})
	}
}
