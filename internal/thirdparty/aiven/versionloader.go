package aiven

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
)

// ServiceKey identifies an Aiven service. ServiceType doubles as the prefix of the
// metadata key Aiven reports the running version under, e.g. "opensearch_version".
type ServiceKey struct {
	Project     string
	ServiceName string
	ServiceType string
}

// ServiceVersion returns the version Aiven reports for a service. Aiven is the sole
// authority for it, so anything short of a usable version is an error rather than an
// absent value the caller has to interpret.
func ServiceVersion(ctx context.Context, key ServiceKey) (*string, error) {
	return fromContext(ctx).versionLoader.Load(ctx, key)
}

type versionDataloader struct {
	aivenClient AivenClient
	log         logrus.FieldLogger
}

func (l versionDataloader) serviceVersions(ctx context.Context, keys []ServiceKey) ([]*string, []error) {
	wg := pool.New().WithContext(ctx)
	rets := make([]*string, len(keys))
	errs := make([]error, len(keys))

	for i, key := range keys {
		wg.Go(func(ctx context.Context) error {
			version, err := l.serviceVersion(ctx, key)
			if err != nil {
				l.log.WithError(err).WithField("project", key.Project).WithField("service", key.ServiceName).Error("fetching Aiven service version")
				errs[i] = err
				return nil
			}

			rets[i] = version
			return nil
		})
	}

	wg.Wait() // #nosec G104 -- goroutines always return nil; per-key errors are collected in errs

	return rets, errs
}

func (l versionDataloader) serviceVersion(ctx context.Context, key ServiceKey) (*string, error) {
	res, err := l.aivenClient.ServiceGet(ctx, key.Project, key.ServiceName)
	if err != nil {
		return nil, err
	}

	metadataKey := key.ServiceType + "_version"
	version, ok := res.Metadata[metadataKey]
	if !ok {
		return nil, fmt.Errorf("aiven reported no %q for service %q", metadataKey, key.ServiceName)
	}

	str, ok := version.(string)
	if !ok {
		return nil, fmt.Errorf("aiven reported a non-string %q for service %q: %T", metadataKey, key.ServiceName, version)
	}

	return &str, nil
}
