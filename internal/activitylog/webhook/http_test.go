package webhook

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"testing"

	"github.com/nais/api/internal/activitylog/webhook/webhooksql"
)

func TestValidateWebhookURL(t *testing.T) {
	for _, tt := range []struct {
		url string
		ok  bool
	}{
		{"https://hooks.example.com/events", true},
		{"https://myapp.intern.mytenant.cloud.nais.io/events", true},
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
			if err := validateWebhookURL(tt.url); (err == nil) != tt.ok {
				t.Errorf("validateWebhookURL() = %v, want valid=%v", err, tt.ok)
			}
		})
	}
}

func TestWebhookDestinationPolicy(t *testing.T) {
	policy := destinationPolicy{internalDomain: "intern.mytenant.cloud.nais.io"}
	for _, tt := range []struct {
		host, ip string
		want     bool
	}{
		{"hooks.example.com", "8.8.8.8", true},
		{"hooks.example.com", "2606:4700:4700::1111", true},
		{"hooks.example.com", "10.1.2.3", false},
		{"myapp.intern.mytenant.cloud.nais.io", "10.1.2.3", true},
		{"myapp.intern.mytenant.cloud.nais.io", "fd12::1", true},
		{"MYAPP.INTERN.MYTENANT.CLOUD.NAIS.IO.", "192.168.1.2", true},
		{"myapp.intern.other.cloud.nais.io", "10.1.2.3", false},
		{"myapp.intern.mytenant.cloud.nais.io.evil.example", "10.1.2.3", false},
		{"myapp.evilintern.mytenant.cloud.nais.io", "10.1.2.3", false},
		{"myapp.intern.mytenant.cloud.nais.io", "127.0.0.1", false},
		{"myapp.intern.mytenant.cloud.nais.io", "::1", false},
		{"myapp.intern.mytenant.cloud.nais.io", "::ffff:127.0.0.1", false},
		{"myapp.intern.mytenant.cloud.nais.io", "169.254.169.254", false},
		{"myapp.intern.mytenant.cloud.nais.io", "fd00:ec2::254", false},
		{"myapp.intern.mytenant.cloud.nais.io", "168.63.129.16", false},
		{"myapp.intern.mytenant.cloud.nais.io", "100.100.100.200", false},
		{"myapp.intern.mytenant.cloud.nais.io", "0.0.0.0", false},
		{"myapp.intern.mytenant.cloud.nais.io", "224.0.0.1", false},
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
