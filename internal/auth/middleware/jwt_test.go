package middleware

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/sirupsen/logrus"
)

func TestJWTAuthorized(t *testing.T) {
	now := time.Date(2025, 5, 15, 13, 48, 42, 0, time.UTC)

	tests := map[string]struct {
		inputAudience     string
		inputIssuer       string
		inputZitadelOrgID string
		inputTime         time.Time

		expectedAudience         string
		expectedIssuer           string
		expectedZitadelOrgID     string
		expectedNaisZitadelOrgID string
		expectedTime             time.Time
		expectedError            error
	}{
		"valid token": {
			inputAudience:        "aud",
			inputIssuer:          "http://localhost:1234",
			inputZitadelOrgID:    "org-id",
			expectedAudience:     "aud",
			expectedIssuer:       "http://localhost:1234",
			expectedZitadelOrgID: "org-id",
		},
		"invalid audience": {
			inputAudience:        "invalid-audience",
			inputIssuer:          "http://localhost:1234",
			inputZitadelOrgID:    "org-id",
			expectedAudience:     "aud",
			expectedIssuer:       "http://localhost:1234",
			expectedZitadelOrgID: "org-id",
			expectedError:        jwt.InvalidAudienceError(),
		},
		"nais.io token when enabled": {
			inputAudience: "aud", inputIssuer: "http://localhost:1234", inputZitadelOrgID: "nais-org-id",
			expectedAudience: "aud", expectedIssuer: "http://localhost:1234", expectedZitadelOrgID: "org-id",
			expectedNaisZitadelOrgID: "nais-org-id",
		},
		"nais.io token when disabled": {
			inputAudience: "aud", inputIssuer: "http://localhost:1234", inputZitadelOrgID: "nais-org-id",
			expectedAudience: "aud", expectedIssuer: "http://localhost:1234", expectedZitadelOrgID: "org-id",
			expectedError: jwt.ValidateError(),
		},
		"tenant token when nais.io is enabled": {
			inputAudience: "aud", inputIssuer: "http://localhost:1234", inputZitadelOrgID: "org-id",
			expectedAudience: "aud", expectedIssuer: "http://localhost:1234", expectedZitadelOrgID: "org-id",
			expectedNaisZitadelOrgID: "nais-org-id",
		},
		"another organization when nais.io is enabled": {
			inputAudience: "aud", inputIssuer: "http://localhost:1234", inputZitadelOrgID: "other-org-id",
			expectedAudience: "aud", expectedIssuer: "http://localhost:1234", expectedZitadelOrgID: "org-id",
			expectedNaisZitadelOrgID: "nais-org-id", expectedError: jwt.ValidateError(),
		},
		"nais.io token with invalid audience": {
			inputAudience: "other-aud", inputIssuer: "http://localhost:1234", inputZitadelOrgID: "nais-org-id",
			expectedAudience: "aud", expectedIssuer: "http://localhost:1234", expectedZitadelOrgID: "org-id",
			expectedNaisZitadelOrgID: "nais-org-id", expectedError: jwt.InvalidAudienceError(),
		},
		"nais.io token with invalid issuer": {
			inputAudience: "aud", inputIssuer: "http://other-issuer", inputZitadelOrgID: "nais-org-id",
			expectedAudience: "aud", expectedIssuer: "http://localhost:1234", expectedZitadelOrgID: "org-id",
			expectedNaisZitadelOrgID: "nais-org-id", expectedError: jwt.InvalidIssuerError(),
		},
		"nais.io expired token": {
			inputAudience: "aud", inputIssuer: "http://localhost:1234", inputZitadelOrgID: "nais-org-id",
			inputTime:        now.Add(-time.Hour),
			expectedAudience: "aud", expectedIssuer: "http://localhost:1234", expectedZitadelOrgID: "org-id",
			expectedNaisZitadelOrgID: "nais-org-id", expectedError: jwt.TokenExpiredError(),
		},
		"missing organization when nais.io is enabled": {
			inputAudience: "aud", inputIssuer: "http://localhost:1234",
			expectedAudience: "aud", expectedIssuer: "http://localhost:1234", expectedZitadelOrgID: "org-id",
			expectedNaisZitadelOrgID: "nais-org-id", expectedError: jwt.ValidateError(),
		},
		"invalid issuer": {
			inputAudience:        "aud",
			inputIssuer:          "http://invalid-issuer",
			inputZitadelOrgID:    "org-id",
			expectedAudience:     "aud",
			expectedIssuer:       "http://localhost:1234",
			expectedZitadelOrgID: "org-id",
			expectedError:        jwt.InvalidIssuerError(),
		},
		"invalid zitadel org ID": {
			inputAudience:        "aud",
			inputIssuer:          "http://localhost:1234",
			inputZitadelOrgID:    "invalid-org-id",
			expectedAudience:     "aud",
			expectedIssuer:       "http://localhost:1234",
			expectedZitadelOrgID: "org-id",
			expectedError:        jwt.ValidateError(),
		},
		"expired token": {
			inputAudience:        "aud",
			inputIssuer:          "http://localhost:1234",
			inputZitadelOrgID:    "org-id",
			inputTime:            now.Add(-time.Hour),
			expectedAudience:     "aud",
			expectedIssuer:       "http://localhost:1234",
			expectedZitadelOrgID: "org-id",
			expectedTime:         now,
			expectedError:        jwt.TokenExpiredError(),
		},
	}

	log := logrus.New()
	if !testing.Verbose() {
		log.SetOutput(io.Discard)
	}

	defaultTime := func(t time.Time) time.Time {
		if t.IsZero() {
			return now
		}
		return t
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			jwkSet, err := newJwkSet("1234")
			if err != nil {
				t.Fatal(err)
			}

			cache, err := jwk.NewCache(t.Context(), httprc.NewClient(
				httprc.WithHTTPClient(mockJWKSetClient(jwkSet)),
			))
			if err != nil {
				t.Fatal(err)
			}
			cache.Register(t.Context(), test.expectedIssuer)

			mw := jwtAuth{
				issuer:           test.expectedIssuer,
				audience:         test.expectedAudience,
				zitadelOrgID:     test.expectedZitadelOrgID,
				naisZitadelOrgID: test.expectedNaisZitadelOrgID,
				jwksURL:          test.expectedIssuer,
				jwksCache:        cache,
				log:              log,
				now: func() time.Time {
					return defaultTime(test.expectedTime)
				},
			}

			token := token(defaultTime(test.inputTime), time.Minute)
			token.Set("aud", test.inputAudience)
			token.Set("iss", test.inputIssuer)
			token.Set("urn:zitadel:iam:user:resourceowner:id", test.inputZitadelOrgID)
			tok, err := token.sign(jwkSet)
			if err != nil {
				t.Fatal(err)
			}

			_, err = mw.validate(t.Context(), tok)
			if err != nil {
				if !errors.Is(err, test.expectedError) {
					t.Fatal(err)
				}
			} else if test.expectedError != nil {
				t.Fatalf("expected error %v, got nil", test.expectedError)
			}
		})
	}
}

func TestJWTAuthorizedSkipsUnrelatedIssuersBeforeJWKSLookup(t *testing.T) {
	now := time.Date(2025, 5, 15, 13, 48, 42, 0, time.UTC)
	const issuer = "http://localhost:1234"

	jwkSet, err := newJwkSet("1234")
	if err != nil {
		t.Fatal(err)
	}
	foreignKeySet, err := newJwkSet("foreign")
	if err != nil {
		t.Fatal(err)
	}

	var fetches atomic.Int32
	inner := mockJWKSetClient(jwkSet).Transport
	httpClient := &http.Client{Transport: RoundTripFn(func(req *http.Request) *http.Response {
		fetches.Add(1)
		resp, _ := inner.RoundTrip(req)
		return resp
	})}

	cache, err := jwk.NewCache(t.Context(), httprc.NewClient(httprc.WithHTTPClient(httpClient)))
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Register(t.Context(), issuer); err != nil {
		t.Fatal(err)
	}

	mw := jwtAuth{
		issuer:       issuer,
		audience:     "aud",
		zitadelOrgID: "org-id",
		jwksURL:      issuer,
		jwksCache:    cache,
		log:          logrus.New(),
		now:          func() time.Time { return now },
	}

	signed := func(set jwk.Set, iss string) string {
		tok := token(now, time.Minute)
		tok.Set("aud", "aud")
		tok.Set("urn:zitadel:iam:user:resourceowner:id", "org-id")
		if iss != "" {
			tok.Set("iss", iss)
		}
		s, err := tok.sign(set)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	// Prime the cache with a valid token from our own issuer.
	if _, err := mw.validate(t.Context(), signed(jwkSet, issuer)); err != nil {
		t.Fatalf("priming validate: %v", err)
	}
	primed := fetches.Load()

	unrelated := map[string]string{
		"foreign issuer, foreign key": signed(foreignKeySet, "http://foreign-issuer"),
		"foreign issuer, own key":     signed(jwkSet, "http://foreign-issuer"),
		"missing issuer":              signed(foreignKeySet, ""),
		"malformed token":             "not-a-jwt",
		"opaque token":                "a.b.c",
	}
	for name, tok := range unrelated {
		t.Run(name, func(t *testing.T) {
			if _, err := mw.validate(t.Context(), tok); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}

	if got := fetches.Load() - primed; got != 0 {
		t.Fatalf("expected no JWKS fetches for unrelated tokens, got %d", got)
	}
}

type RoundTripFn func(req *http.Request) *http.Response

func (f RoundTripFn) RoundTrip(req *http.Request) (*http.Response, error) { return f(req), nil }

func mockJWKSetClient(jwks jwk.Set) *http.Client {
	return &http.Client{
		Transport: RoundTripFn(func(req *http.Request) *http.Response {
			publicKeys, err := jwk.PublicSetOf(jwks)
			if err != nil {
				panic(err)
			}

			b, err := json.Marshal(publicKeys)
			if err != nil {
				panic(err)
			}

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(b)),
				Header:     make(http.Header),
			}
		}),
	}
}

type Token struct {
	jwt.Token
}

func newJwkSet(kid string) (jwk.Set, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	key, err := jwk.Import(privateKey)
	if err != nil {
		return nil, err
	}

	key.Set(jwk.AlgorithmKey, jwa.ES256())
	key.Set(jwk.KeyTypeKey, jwa.EC())
	key.Set(jwk.KeyIDKey, kid)
	privateKeys := jwk.NewSet()
	privateKeys.AddKey(key)
	return privateKeys, nil
}

func token(iat time.Time, exp time.Duration) *Token {
	jwt.Settings(jwt.WithFlattenAudience(true))
	expiry := iat.Add(exp)
	accessToken := jwt.New()
	accessToken.Set("iat", iat.Unix())
	accessToken.Set("exp", expiry.Unix())
	return &Token{accessToken}
}

func (t *Token) sign(set jwk.Set) (string, error) {
	signer, ok := set.Key(0)
	if !ok {
		return "", fmt.Errorf("could not get signer")
	}

	tok, err := t.Clone()
	if err != nil {
		return "", err
	}
	signedToken, err := jwt.Sign(tok, jwt.WithKey(jwa.ES256(), signer))
	if err != nil {
		return "", err
	}
	return string(signedToken), nil
}
