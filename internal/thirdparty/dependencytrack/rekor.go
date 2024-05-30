package dependencytrack

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	dsse "github.com/sigstore/rekor/pkg/types/dsse/v0.0.1"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/nais/api/internal/graph/model"
	"github.com/patrickmn/go-cache"
	rclient "github.com/sigstore/rekor/pkg/generated/client"
	"github.com/sigstore/rekor/pkg/generated/client/entries"
	"github.com/sigstore/rekor/pkg/generated/models"
	"github.com/sigstore/rekor/pkg/pki"
	"github.com/sirupsen/logrus"
)

type ExtensionIdentifier string

const (
	OIDCIssuer                          ExtensionIdentifier = "1.3.6.1.4.1.57264.1.1"
	GithubWorkflowTrigger               ExtensionIdentifier = "1.3.6.1.4.1.57264.1.2"
	GithubWorkflowSHA                   ExtensionIdentifier = "1.3.6.1.4.1.57264.1.3"
	GithubWorkflowName                  ExtensionIdentifier = "1.3.6.1.4.1.57264.1.4"
	GithubWorkflowRepository            ExtensionIdentifier = "1.3.6.1.4.1.57264.1.5"
	GithubWorkflowRef                   ExtensionIdentifier = "1.3.6.1.4.1.57264.1.6"
	BuildSignerURI                      ExtensionIdentifier = "1.3.6.1.4.1.57264.1.9"
	BuildSignerDigest                   ExtensionIdentifier = "1.3.6.1.4.1.57264.1.10"
	RunnerEnvironment                   ExtensionIdentifier = "1.3.6.1.4.1.57264.1.11"
	SourceRepositoryURI                 ExtensionIdentifier = "1.3.6.1.4.1.57264.1.12"
	SourceRepositoryDigest              ExtensionIdentifier = "1.3.6.1.4.1.57264.1.13"
	SourceRepositoryRef                 ExtensionIdentifier = "1.3.6.1.4.1.57264.1.14"
	SourceRepositoryIdentifier          ExtensionIdentifier = "1.3.6.1.4.1.57264.1.15"
	SourceRepositoryOwnerURI            ExtensionIdentifier = "1.3.6.1.4.1.57264.1.16"
	SourceRepositoryOwnerIdentifier     ExtensionIdentifier = "1.3.6.1.4.1.57264.1.17"
	BuildConfigURI                      ExtensionIdentifier = "1.3.6.1.4.1.57264.1.18"
	BuildConfigDigest                   ExtensionIdentifier = "1.3.6.1.4.1.57264.1.19"
	BuildTrigger                        ExtensionIdentifier = "1.3.6.1.4.1.57264.1.20"
	RunInvocationURI                    ExtensionIdentifier = "1.3.6.1.4.1.57264.1.21"
	SourceRepositoryVisibilityAtSigning ExtensionIdentifier = "1.3.6.1.4.1.57264.1.22"
)

func (e ExtensionIdentifier) String() string {
	return string(e)
}

type RekorClient interface {
	GetRekorMetadata(ctx context.Context, rekorLogIndex string) (*model.Rekor, error)
}

type RClient struct {
	cache    *cache.Cache
	client   *rclient.Rekor
	log      logrus.FieldLogger
	rekorURL string
}

func NewRekor(rekorURL string) *RClient {
	transport := client.New(rekorURL, rclient.DefaultBasePath, []string{"https"})
	rekorClient := rclient.New(transport, strfmt.Default)

	return &RClient{
		cache:    cache.New(5*time.Minute, 10*time.Minute),
		client:   rekorClient,
		rekorURL: rekorURL,
	}
}

func (r *RClient) GetRekorMetadata(ctx context.Context, rekorLogIndex string) (*model.Rekor, error) {
	if metadata, found := r.cache.Get(rekorLogIndex); found {
		return metadata.(*model.Rekor), nil
	}
	index, err := strconv.ParseInt(rekorLogIndex, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rekor log index: %w", err)
	}

	params := entries.NewGetLogEntryByIndexParamsWithContext(ctx)
	params.SetLogIndex(index)

	logEntryByIndex, err := r.client.Entries.GetLogEntryByIndex(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get log entry by index: %w", err)
	}

	metadata, err := r.parseRekorMetadata(logEntryByIndex.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rekor metadata: %w", err)
	}

	metadata.LogIndex = rekorLogIndex

	r.cache.Set(rekorLogIndex, metadata, cache.DefaultExpiration)
	return metadata, nil
}

func (r *RClient) parseRekorMetadata(rekorLogIndex models.LogEntry) (*model.Rekor, error) {
	entryAnon, found := func(m models.LogEntry) (models.LogEntryAnon, bool) {
		for k := range m {
			return m[k], true
		}
		return models.LogEntryAnon{}, false
	}(rekorLogIndex)

	if !found {
		return nil, fmt.Errorf("log entry not found")
	}

	canonicalValue, err := logEntryToPubKey(entryAnon)
	if err != nil {
		log.Fatalf("failed to get verifier: %v", err)
	}

	metadata, err := certToRekorMetadata(canonicalValue, *entryAnon.IntegratedTime)
	if err != nil {
		log.Fatalf("failed to get rekor metadata: %v", err)
	}

	return metadata, nil
}

func logEntryToPubKey(logEntry models.LogEntryAnon) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(logEntry.Body.(string))
	if err != nil {
		return nil, fmt.Errorf("failed to decode log entry: %w", err)
	}

	logEntryPayload := struct {
		Spec json.RawMessage `json:"spec"`
	}{}
	err = json.Unmarshal(decoded, &logEntryPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal log entry payload: %w", err)
	}

	dsseData := dsse.V001Entry{}
	err = json.Unmarshal(logEntryPayload.Spec, &dsseData.DSSEObj)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal intoto data: %w", err)
	}

	verifiers, err := dsseData.Verifiers()
	if err != nil {
		return nil, fmt.Errorf("failed to get verifiers: %w", err)
	}

	canonicalValue, found := func(m []pki.PublicKey) ([]byte, bool) {
		for k := range m {
			canon, err := m[k].CanonicalValue()
			if err != nil {
				return nil, false
			}
			return canon, true
		}
		return nil, false
	}(verifiers)

	if !found {
		return nil, fmt.Errorf("verifier not found")
	}

	return canonicalValue, nil
}

func certToRekorMetadata(canonicalValue []byte, integratedTime int64) (*model.Rekor, error) {
	rekorMetadata := &model.Rekor{}
	for block, rest := pem.Decode(canonicalValue); block != nil; block, rest = pem.Decode(rest) {
		switch block.Type {
		case "CERTIFICATE":
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse certificate: %w", err)
			}

			for _, ext := range cert.Extensions {
				switch ext.Id.String() {
				case OIDCIssuer.String():
					rekorMetadata.OIDCIssuer = string(ext.Value)
				case GithubWorkflowName.String():
					rekorMetadata.GitHubWorkflowName = string(ext.Value)
				case GithubWorkflowRef.String():
					rekorMetadata.GitHubWorkflowRef = string(ext.Value)
				case BuildTrigger.String():
					rekorMetadata.BuildTrigger = removeNoneGraphicChars(string(ext.Value))
				case RunInvocationURI.String():
					rekorMetadata.RunInvocationURI = trimBeforeSubstring(string(ext.Value), "https://")
				case RunnerEnvironment.String():
					rekorMetadata.RunnerEnvironment = removeNoneGraphicChars(string(ext.Value))
				case SourceRepositoryOwnerURI.String():
					rekorMetadata.SourceRepositoryOwnerURI = removeNoneGraphicChars(string(ext.Value))
				case BuildConfigURI.String():
					rekorMetadata.BuildConfigURI = trimBeforeSubstring(string(ext.Value), "https://")
				}
			}
			rekorMetadata.IntegratedTime = int(integratedTime)
		default:
			log.Fatalf("Unknown PEM block type: %s", block.Type)
		}
	}

	return rekorMetadata, nil
}

func removeNoneGraphicChars(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsGraphic(r) {
			return r
		}
		return -1
	}, s)
}

func trimBeforeSubstring(input, substring string) string {
	index := strings.Index(input, substring)
	if index == -1 {
		return input
	}
	return input[index:]
}
