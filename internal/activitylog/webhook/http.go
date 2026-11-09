package webhook

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
)

func validateWebhookURL(raw string, allowInsecure bool) error {
	u, err := url.Parse(raw)
	if allowInsecure {
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
			return errors.New("webhook URL must be an HTTP or HTTPS URL without credentials or a fragment")
		}
		return nil
	}
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return errors.New("webhook URL must be an HTTPS URL without credentials or a fragment")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if net.ParseIP(host) != nil || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.ContainsAny(host, "%\\") {
		return errors.New("webhook URL must use a hostname, not a local address or IP literal")
	}
	return nil
}

func (d *Dispatcher) validateURL(raw string) error {
	return validateWebhookURL(raw, d != nil && d.allowInsecureURLs)
}

type destinationPolicy struct {
	internalDomain string
	lookup         func(context.Context, string, string) ([]netip.Addr, error)
	dial           func(context.Context, string, string) (net.Conn, error)
}

var blockedDestinationPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("168.63.129.16/32"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("fd00:ec2::254/128"),
}

func (p destinationPolicy) allows(host string, address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsLoopback() || address.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range blockedDestinationPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	internal := p.internalDomain != "" && strings.HasSuffix(host, "."+p.internalDomain)
	return !address.IsPrivate() || internal
}

func (p destinationPolicy) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses, err := p.lookup(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(addresses) == 0 {
		return nil, errors.New("webhook hostname has no addresses")
	}
	for _, ip := range addresses {
		if !p.allows(host, ip) {
			return nil, errors.New("webhook destination is not allowed")
		}
	}
	var dialErr error
	for _, ip := range addresses {
		conn, err := p.dial(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		dialErr = err
	}
	return nil, dialErr
}

type webhookTransport struct {
	transport         *http.Transport
	allowInsecureURLs bool
}

func (t webhookTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := validateWebhookURL(req.URL.String(), t.allowInsecureURLs); err != nil {
		return nil, err
	}
	return t.transport.RoundTrip(req)
}

func newWebhookHTTPClient(tenant string, allowInsecureURLs bool) *http.Client {
	policy := destinationPolicy{
		lookup: net.DefaultResolver.LookupNetIP,
		dial:   (&net.Dialer{Timeout: defaultTimeout}).DialContext,
	}
	if tenant != "" {
		policy.internalDomain = fmt.Sprintf("intern.%s.cloud.nais.io", strings.ToLower(tenant))
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	if !allowInsecureURLs {
		transport.DialContext = policy.dialContext
	}
	return &http.Client{
		Timeout:       defaultTimeout,
		Transport:     webhookTransport{transport: transport, allowInsecureURLs: allowInsecureURLs},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
