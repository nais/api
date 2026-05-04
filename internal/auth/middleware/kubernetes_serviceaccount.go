package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/serviceaccount"
	"github.com/nais/api/internal/slug"
	"github.com/sirupsen/logrus"
	"github.com/zitadel/oidc/v3/pkg/client"
)

// KubernetesServiceAccountAudience is the only audience the API will accept on Kubernetes-issued projected
// ServiceAccount tokens. Workloads must request a token with this audience to authenticate.
const KubernetesServiceAccountAudience = "nais"

// KubernetesIssuer represents one trusted Kubernetes cluster's OIDC issuer for projected ServiceAccount tokens.
type KubernetesIssuer struct {
	// Environment is the name of the Nais environment (cluster) this issuer corresponds to.
	Environment string

	// IssuerURL is the OIDC issuer URL (the value that will appear in the `iss` claim and where OIDC discovery
	// is performed).
	IssuerURL string
}

// k8sSAAuth is the request-handling state for the Kubernetes SA authentication middleware.
type k8sSAAuth struct {
	// issuers maps `iss` claim value -> environment + jwks URL.
	issuers map[string]struct {
		environment string
		jwksURL     string
	}
	jwksCache *jwk.Cache
	log       logrus.FieldLogger
}

// KubernetesServiceAccountAuthentication builds an HTTP middleware that authenticates requests using a Kubernetes
// projected ServiceAccount token.
//
// The middleware looks for a Bearer token that parses as a JWT (three dot-separated segments). If found, it is
// validated against the issuer JWKS of one of the trusted clusters. On success, the binding (env, namespace, sa
// name) is looked up and the request is authenticated as the bound Nais service account.
//
// If the token does not look like a JWT, or validation fails, the middleware passes the request to the next
// handler unchanged so that other authentication mechanisms (such as opaque bearer tokens) get a chance.
func KubernetesServiceAccountAuthentication(ctx context.Context, issuers []KubernetesIssuer, log logrus.FieldLogger) (func(next http.Handler) http.Handler, error) {
	if len(issuers) == 0 {
		// No trusted issuers — return a no-op middleware. This makes the middleware safe to wire up before any
		// clusters are configured.
		return func(next http.Handler) http.Handler { return next }, nil
	}

	httpClient := httprc.NewClient()
	cache, err := jwk.NewCache(ctx, httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating jwks cache: %w", err)
	}

	idx := make(map[string]struct {
		environment string
		jwksURL     string
	}, len(issuers))

	for _, iss := range issuers {
		if iss.IssuerURL == "" || iss.Environment == "" {
			return nil, fmt.Errorf("invalid kubernetes issuer entry: environment=%q, issuer=%q", iss.Environment, iss.IssuerURL)
		}
		disc, err := client.Discover(ctx, iss.IssuerURL, http.DefaultClient)
		if err != nil {
			// Don't fail middleware setup just because one cluster's discovery is unavailable. Log and skip; we
			// can re-try at request time by re-registering. But for now: fail loudly so misconfiguration is
			// obvious.
			return nil, fmt.Errorf("discovering oidc for cluster %q (%s): %w", iss.Environment, iss.IssuerURL, err)
		}
		if err := cache.Register(ctx, disc.JwksURI); err != nil {
			return nil, fmt.Errorf("registering jwks for cluster %q: %w", iss.Environment, err)
		}
		idx[iss.IssuerURL] = struct {
			environment string
			jwksURL     string
		}{
			environment: iss.Environment,
			jwksURL:     disc.JwksURI,
		}
	}

	auth := &k8sSAAuth{
		issuers:   idx,
		jwksCache: cache,
		log:       log,
	}

	return auth.handler, nil
}

// looksLikeJWT returns true if the value has three dot-separated, non-empty segments — which is the structural
// shape of a compact-serialized JWS/JWT. This is a cheap pre-check used to decide whether to attempt JWT
// validation, or to fall through to the next authentication middleware.
func looksLikeJWT(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
	}
	return true
}

func (k *k8sSAAuth) handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := BearerAuth(r)
		if !ok || !looksLikeJWT(token) {
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()

		// Inspect the unverified token to find the issuer and look up the matching cluster.
		unverified, err := jwt.ParseInsecure([]byte(token))
		if err != nil {
			k.log.WithError(err).Debug("k8s sa auth: parse insecure")
			next.ServeHTTP(w, r)
			return
		}
		issuer, ok := unverified.Issuer()
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		entry, ok := k.issuers[issuer]
		if !ok {
			// Unknown issuer — not a token for us.
			next.ServeHTTP(w, r)
			return
		}

		jwks, err := k.jwksCache.Lookup(ctx, entry.jwksURL)
		if err != nil {
			k.log.WithError(err).WithField("issuer", issuer).Debug("k8s sa auth: jwks lookup")
			next.ServeHTTP(w, r)
			return
		}

		verified, err := jwt.Parse([]byte(token),
			jwt.WithKeySet(jwks),
			jwt.WithIssuer(issuer),
			jwt.WithAudience(KubernetesServiceAccountAudience),
		)
		if err != nil {
			k.log.WithError(err).WithField("issuer", issuer).Debug("k8s sa auth: validate")
			next.ServeHTTP(w, r)
			return
		}

		ns, name, k8sUID, ok := extractK8sServiceAccount(verified)
		if !ok {
			k.log.Debug("k8s sa auth: missing kubernetes.io claim")
			next.ServeHTTP(w, r)
			return
		}

		teamSlug := slug.Slug(ns)

		result, err := serviceaccount.AuthenticateKubernetesServiceAccount(ctx, entry.environment, teamSlug, name, k8sUID)
		if err != nil {
			k.log.WithError(err).WithFields(logrus.Fields{
				"environment": entry.environment,
				"team":        ns,
				"k8s_sa":      name,
				"k8s_sa_uid":  k8sUID,
			}).Debug("k8s sa auth: lookup binding")
			next.ServeHTTP(w, r)
			return
		}

		roles, err := authz.ForServiceAccount(ctx, result.ServiceAccount.UUID)
		if err != nil {
			k.log.WithError(err).Debug("k8s sa auth: roles")
			next.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r.WithContext(authz.ContextWithActor(ctx, result.ServiceAccount, roles)))
	})
}

// extractK8sServiceAccount pulls (namespace, sa-name, sa-uid) out of the standard "kubernetes.io" claim that
// projected K8s SA tokens carry. Returns ok=false if any of the values are missing or malformed.
func extractK8sServiceAccount(t jwt.Token) (namespace, name string, uid uuid.UUID, ok bool) {
	var raw any
	if err := t.Get("kubernetes.io", &raw); err != nil {
		return "", "", uuid.Nil, false
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return "", "", uuid.Nil, false
	}
	ns, _ := m["namespace"].(string)
	saRaw, _ := m["serviceaccount"].(map[string]any)
	if ns == "" || saRaw == nil {
		return "", "", uuid.Nil, false
	}
	saName, _ := saRaw["name"].(string)
	saUIDStr, _ := saRaw["uid"].(string)
	if saName == "" || saUIDStr == "" {
		return "", "", uuid.Nil, false
	}
	parsedUID, err := uuid.Parse(saUIDStr)
	if err != nil {
		return "", "", uuid.Nil, false
	}
	return ns, saName, parsedUID, true
}
