package middleware

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/sirupsen/logrus"
)

func TestJWTAuthorized(t *testing.T) {
	now := time.Date(2025, 05, 15, 13, 48, 42, 0, time.UTC)

	tests := map[string]struct {
		inputAudience string
		inputIssuer   string
		inputTime     time.Time

		expectedAudience string
		expectedIssuer   string
		expectedTime     time.Time
		expectedError    error
	}{
		"valid token": {
			inputAudience:    "aud",
			inputIssuer:      "http://localhost:1234",
			expectedAudience: "aud",
			expectedIssuer:   "http://localhost:1234",
		},
		"invalid audience": {
			inputAudience:    "invalid-audience",
			inputIssuer:      "http://localhost:1234",
			expectedAudience: "aud",
			expectedIssuer:   "http://localhost:1234",
			expectedError:    jwt.InvalidAudienceError(),
		},
		"invalid issuer": {
			inputAudience:    "aud",
			inputIssuer:      "http://invalid-issuer",
			expectedAudience: "aud",
			expectedIssuer:   "http://localhost:1234",
			expectedError:    jwt.InvalidIssuerError(),
		},
		"expired token": {
			inputAudience:    "aud",
			inputIssuer:      "http://localhost:1234",
			inputTime:        now.Add(-time.Hour),
			expectedAudience: "aud",
			expectedIssuer:   "http://localhost:1234",
			expectedTime:     now,
			expectedError:    jwt.TokenExpiredError(),
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

			cache, err := jwk.NewCache(context.Background(), httprc.NewClient(
				httprc.WithHTTPClient(mockJWKSetClient(jwkSet)),
			))
			cache.Register(context.Background(), test.expectedIssuer)

			mw := jwtAuth{
				issuer:    test.expectedIssuer,
				audience:  test.expectedAudience,
				jwksURL:   test.expectedIssuer,
				jwksCache: cache,
				log:       log,
				now: func() time.Time {
					return defaultTime(test.expectedTime)
				},
			}

			token := token(defaultTime(test.inputTime), time.Minute)
			token.Set("aud", test.inputAudience)
			token.Set("iss", test.inputIssuer)
			tok, err := token.sign(jwkSet)
			if err != nil {
				t.Fatal(err)
			}

			_, err = mw.validate(context.Background(), tok)
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
