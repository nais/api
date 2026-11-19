package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/nais/api/internal/activitylog/webhook/webhooksql"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/workload/application"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

func TestLocalWebhookURLsRequireExplicitOptIn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "/path")
			w.WriteHeader(http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	localURL := strings.Replace(server.URL, "127.0.0.1", "localhost", 1) + "/path"
	for _, allowInsecure := range []bool{false, true} {
		t.Run(map[bool]string{false: "secure default", true: "explicit opt-in"}[allowInsecure], func(t *testing.T) {
			d := &Dispatcher{allowInsecureURLs: allowInsecure}
			if err := d.validateURL(localURL); (err == nil) != allowInsecure {
				t.Fatalf("validateURL() = %v, allowInsecure=%v", err, allowInsecure)
			}
			client := newWebhookHTTPClient(allowInsecure, nil)
			defer client.CloseIdleConnections()
			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, localURL, strings.NewReader("{}"))
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.Do(req)
			if !allowInsecure {
				if err == nil {
					response.Body.Close()
					t.Fatal("secure client must reject local HTTP destinations")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusNoContent {
				t.Errorf("status = %d, want %d", response.StatusCode, http.StatusNoContent)
			}
			req, err = http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/redirect", nil)
			if err != nil {
				t.Fatal(err)
			}
			response, err = client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusFound {
				t.Errorf("redirect was followed: status = %d", response.StatusCode)
			}
		})
	}

	var dispatcher *Dispatcher
	if err := dispatcher.validateURL(localURL); err == nil {
		t.Error("missing dispatcher must retain secure URL validation")
	}
}

func TestInsecureWebhookURLValidation(t *testing.T) {
	for _, tt := range []struct {
		url     string
		wantErr bool
	}{
		{"http://localhost:32424/path", false},
		{"http://127.0.0.1:32424/path", false},
		{"https://[::1]:32424/path", false},
		{"http://10.0.0.1/path", false},
		{"file:///path", true},
		{"http:///path", true},
		{"http://localhost/path#fragment", true},
	} {
		t.Run(tt.url, func(t *testing.T) {
			if err := validateWebhookURL(tt.url, true); (err != nil) != tt.wantErr {
				t.Errorf("validateWebhookURL() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateWebhookURL(t *testing.T) {
	for _, tt := range []struct {
		url string
		ok  bool
	}{
		{"https://hooks.example.com/events", true},
		{"https://myapp.internal.example.com/events", true},
		{"http://hooks.example.com", false},
		{"https://user:password@hooks.example.com", false},
		{"https://hooks.example.com/#secret", false},
		{"https://127.0.0.1/", false},
		{"https://[::1]/", false},
		{"https://localhost/", false},
		{"https://foo.localhost/", false},
		{"https:///events", false},
	} {
		t.Run(tt.url, func(t *testing.T) {
			if err := validateWebhookURL(tt.url, false); (err == nil) != tt.ok {
				t.Errorf("validateWebhookURL() = %v, want valid=%v", err, tt.ok)
			}
		})
	}
}

func TestWebhookDestinationPolicy(t *testing.T) {
	policy := destinationPolicy{}
	for _, tt := range []struct {
		host, ip string
		want     bool
	}{
		{"hooks.example.com", "8.8.8.8", true},
		{"hooks.example.com", "2606:4700:4700::1111", true},
		{"hooks.example.com", "10.1.2.3", false},
		{"myapp.intern.mytenant.cloud.nais.io", "10.1.2.3", false},
		{"hooks.example.com", "fd12::1", false},
		{"hooks.example.com", "192.168.1.2", false},
		{"hooks.example.com", "127.0.0.1", false},
		{"hooks.example.com", "::1", false},
		{"hooks.example.com", "::ffff:127.0.0.1", false},
		{"hooks.example.com", "169.254.169.254", false},
		{"hooks.example.com", "fd00:ec2::254", false},
		{"hooks.example.com", "168.63.129.16", false},
		{"hooks.example.com", "100.100.100.200", false},
		{"hooks.example.com", "0.0.0.0", false},
		{"hooks.example.com", "224.0.0.1", false},
		{"hooks.example.com", "64:ff9b::a00:1", false},
	} {
		t.Run(tt.host+"/"+tt.ip, func(t *testing.T) {
			if got := policy.allows(tt.host, netip.MustParseAddr(tt.ip)); got != tt.want {
				t.Errorf("allows() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWebhookDialPinsAddressesAndRechecksDNS(t *testing.T) {
	lookups, dials := 0, 0
	policy := destinationPolicy{
		lookup: func(context.Context, string, string) ([]netip.Addr, error) {
			lookups++
			if lookups == 1 {
				return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
			}
			return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
		},
		dial: func(_ context.Context, _, address string) (net.Conn, error) {
			dials++
			if address != "8.8.8.8:443" {
				t.Errorf("dialed %q instead of the validated IP", address)
			}
			return nil, errors.New("test dial")
		},
	}
	_, _ = policy.dialContext(t.Context(), "tcp", "hooks.example.com:443")
	if _, err := policy.dialContext(t.Context(), "tcp", "hooks.example.com:443"); err == nil {
		t.Error("DNS rebinding to loopback must fail")
	}
	if lookups != 2 || dials != 1 {
		t.Errorf("lookups=%d, dials=%d; want 2, 1", lookups, dials)
	}
	policy.lookup = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.0.0.1")}, nil
	}
	if _, err := policy.dialContext(t.Context(), "tcp", "hooks.example.com:443"); err == nil || dials != 1 {
		t.Error("mixed public and private answers must fail before dialing")
	}
}

func TestWebhookDestinationPolicyUsesInternalIngresses(t *testing.T) {
	fixtures := fstest.MapFS{}
	for _, class := range []string{"nais-ingress", "internal-haproxy", "nais-ingress-external", "nais-ingress-fa", "unknown", ""} {
		name := class
		if name == "" {
			name = "no-class"
		}
		ingress := &netv1.Ingress{
			TypeMeta:   metav1.TypeMeta{APIVersion: "networking.k8s.io/v1", Kind: "Ingress"},
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "team"},
			Spec: netv1.IngressSpec{
				Rules: []netv1.IngressRule{{Host: name + ".custom.example.com"}},
			},
		}
		if class != "" {
			ingress.Spec.IngressClassName = ptr.To(class)
		}
		data, err := json.Marshal(ingress)
		if err != nil {
			t.Fatal(err)
		}
		fixtures["test/team/"+name+".yaml"] = &fstest.MapFile{Data: data}
	}
	log, _ := logrustest.NewNullLogger()
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := watcher.NewManager(scheme, kubernetes.ClusterConfigMap{"test": nil}, log, watcher.WithClientCreator(fake.Clients(fixtures)))
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Stop()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ingressWatcher := application.NewIngressWatcher(ctx, mgr)
	waitCtx, waitCancel := context.WithTimeout(ctx, 5*time.Second)
	defer waitCancel()
	if !mgr.WaitForReady(waitCtx) {
		t.Fatal("timed out waiting for ingress watcher")
	}

	policy := destinationPolicy{ingressWatcher: ingressWatcher}
	for _, tt := range []struct {
		host, ip string
		want     bool
	}{
		{"nais-ingress.custom.example.com", "10.1.2.3", true},
		{"internal-haproxy.custom.example.com", "192.168.1.2", true},
		{"NAIS-INGRESS.CUSTOM.EXAMPLE.COM.", "fd12::1", true},
		{"nais-ingress-external.custom.example.com", "10.1.2.3", false},
		{"nais-ingress-fa.custom.example.com", "10.1.2.3", false},
		{"unknown.custom.example.com", "10.1.2.3", false},
		{"no-class.custom.example.com", "10.1.2.3", false},
		{"unregistered.custom.example.com", "10.1.2.3", false},
		{"nais-ingress.custom.example.com.evil.example", "10.1.2.3", false},
		{"sub.nais-ingress.custom.example.com", "10.1.2.3", false},
		{"nais-ingress.custom.example.com", "127.0.0.1", false},
		{"nais-ingress.custom.example.com", "::1", false},
		{"nais-ingress.custom.example.com", "169.254.169.254", false},
		{"nais-ingress.custom.example.com", "fd00:ec2::254", false},
		{"nais-ingress.custom.example.com", "168.63.129.16", false},
		{"nais-ingress.custom.example.com", "100.100.100.200", false},
	} {
		t.Run(tt.host+"/"+tt.ip, func(t *testing.T) {
			if got := policy.allows(tt.host, netip.MustParseAddr(tt.ip)); got != tt.want {
				t.Errorf("allows() = %v, want %v", got, tt.want)
			}
		})
	}

	client, err := ingressWatcher.SystemAuthenticatedClient(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	ingresses := client.Namespace("team")
	ingress, err := ingresses.Get(ctx, "nais-ingress", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ingress.SetName("dynamic")
	ingress.SetResourceVersion("")
	setHost := func(host string) {
		t.Helper()
		if err := unstructured.SetNestedSlice(ingress.Object, []any{map[string]any{"host": host}}, "spec", "rules"); err != nil {
			t.Fatal(err)
		}
	}
	waitFor := func(description string, ready func() bool) {
		t.Helper()
		waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for !ready() {
			select {
			case <-waitCtx.Done():
				t.Fatalf("timed out waiting for %s", description)
			case <-ticker.C:
			}
		}
	}
	waitForAllowed := func(host string, want bool) {
		t.Helper()
		waitFor(host+" allowance to change", func() bool {
			return policy.allows(host, netip.MustParseAddr("10.1.2.3")) == want
		})
	}
	update := func() {
		t.Helper()
		ingress, err = ingresses.Update(ctx, ingress, metav1.UpdateOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}

	setHost("created.custom.example.com")
	ingress, err = ingresses.Create(ctx, ingress, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	waitForAllowed("created.custom.example.com", true)

	setHost("updated.custom.example.com")
	update()
	waitForAllowed("created.custom.example.com", false)
	waitForAllowed("updated.custom.example.com", true)

	if err := unstructured.SetNestedField(ingress.Object, "nais-ingress-external", "spec", "ingressClassName"); err != nil {
		t.Fatal(err)
	}
	update()
	waitForAllowed("updated.custom.example.com", false)

	if err := unstructured.SetNestedField(ingress.Object, "internal-haproxy", "spec", "ingressClassName"); err != nil {
		t.Fatal(err)
	}
	update()
	waitForAllowed("updated.custom.example.com", true)

	duplicate := ingress.DeepCopy()
	duplicate.SetName("duplicate")
	duplicate.SetResourceVersion("")
	if _, err := ingresses.Create(ctx, duplicate, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitFor("duplicate ingress to be indexed", func() bool {
		_, err := ingressWatcher.Get("test", "team", duplicate.GetName())
		return err == nil
	})
	if err := ingresses.Delete(ctx, ingress.GetName(), metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	waitFor("original ingress to be removed", func() bool {
		_, err := ingressWatcher.Get("test", "team", ingress.GetName())
		return err != nil
	})
	waitForAllowed("updated.custom.example.com", true)
	if err := ingresses.Delete(ctx, duplicate.GetName(), metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForAllowed("updated.custom.example.com", false)
}

func TestWebhookRedirectsAreFailedAttempts(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			q := newWorkerTestQueries(0)
			calls := 0
			d := newWorkerTestDispatcher(t, q, func(req *http.Request) (*http.Response, error) {
				calls++
				response := workerTestResponse(status)
				response.Header.Set("Location", "https://redirect.example.com")
				return response, nil
			})
			sub := &webhooksql.WebhookSubscription{Url: "https://hooks.example.com", Secret: "test-secret"}
			attempt, err := d.deliver(t.Context(), q, sub, "WEBHOOK_TEST", []byte("{}"), nil)
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || attempt.Success || attempt.ResponseStatus == nil || int(*attempt.ResponseStatus) != status {
				t.Errorf("redirect was not recorded as one failed attempt: calls=%d, attempt=%+v", calls, attempt)
			}
		})
	}
}
