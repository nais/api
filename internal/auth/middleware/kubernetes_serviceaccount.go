package middleware

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
	Environment string `json:"environment"`

	// Issuer is the OIDC issuer URL (the value that will appear in the `iss` claim and where OIDC discovery
	// is performed).
	Issuer string `json:"issuer"`
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
		if iss.Issuer == "" || iss.Environment == "" {
			return nil, fmt.Errorf("invalid kubernetes issuer entry: environment=%q, issuer=%q", iss.Environment, iss.Issuer)
		}
		disc, err := client.Discover(ctx, iss.Issuer, http.DefaultClient)
		if err != nil {
			return nil, fmt.Errorf("discovering oidc for cluster %q (%s): %w", iss.Environment, iss.Issuer, err)
		}
		if err := cache.Register(ctx, disc.JwksURI); err != nil {
			return nil, fmt.Errorf("registering jwks for cluster %q: %w", iss.Environment, err)
		}
		idx[iss.Issuer] = struct {
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
		issuer, ok := getJWTIssuer(token)
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

		_, err = jwt.Parse([]byte(token),
			jwt.WithKeySet(jwks),
			jwt.WithIssuer(issuer),
			jwt.WithAudience(KubernetesServiceAccountAudience),
			jwt.WithValidate(true),
		)
		if err != nil {
			k.log.WithError(err).WithField("issuer", issuer).Debug("k8s sa auth: validate")
			next.ServeHTTP(w, r)
			return
		}

		claims, err := extractK8sServiceAccount(token)
		if !ok {
			k.log.WithError(err).Debug("k8s sa auth: missing kubernetes.io claim")
			next.ServeHTTP(w, r)
			return
		}

		result, err := serviceaccount.AuthenticateKubernetesServiceAccount(
			ctx,
			entry.environment,
			claims.TeamSlug,
			claims.ServiceAccountName,
			claims.ServiceAccountUID,
		)
		if err != nil {
			k.log.WithError(err).WithFields(logrus.Fields{
				"environment": entry.environment,
				"team":        claims.TeamSlug,
				"sa_name":     claims.ServiceAccountName,
				"sa_uid":      claims.ServiceAccountUID,
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

func getJWTIssuer(token string) (string, bool) {
	unverified, err := jwt.ParseInsecure([]byte(token))
	if err != nil {
		return "", false
	}
	return unverified.Issuer()
}

type k8sClaims struct {
	TeamSlug           slug.Slug
	ServiceAccountName string
	ServiceAccountUID  uuid.UUID
}

// extractK8sServiceAccount pulls (namespace, sa-name, sa-uid) out of the standard "kubernetes.io" claim that
// projected K8s SA tokens carry. Make sure to validate the token signature and claims before calling this, as
// it does not perform any verification.
func extractK8sServiceAccount(t string) (*k8sClaims, error) {
	parts := strings.SplitN(t, ".", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("token does not have three parts")
	}

	var claims struct {
		Kubernetes struct {
			Namespace      string `json:"namespace"`
			ServiceAccount struct {
				Name string `json:"name"`
				UID  string `json:"uid"`
			} `json:"serviceaccount"`
		} `json:"kubernetes.io"`
	}

	if err := json.Unmarshal(base64.StdEncoding.DecodeString(parts[1])); err != nil {
		return nil, fmt.Errorf("unmarshaling kubernetes.io claim: %w", err)
	}

	if claims.Kubernetes.Namespace == "" || claims.Kubernetes.ServiceAccount.Name == "" || claims.Kubernetes.ServiceAccount.UID == "" {
		return nil, fmt.Errorf("missing fields in kubernetes.io claim")
	}

	uid, err := uuid.Parse(claims.Kubernetes.ServiceAccount.UID)
	if err != nil {
		return nil, fmt.Errorf("parsing service account UID as UUID: %w", err)
	}

	return &k8sClaims{
		TeamSlug:           slug.Slug(claims.Kubernetes.Namespace),
		ServiceAccountName: claims.Kubernetes.ServiceAccount.Name,
		ServiceAccountUID:  uid,
	}, nil
}
