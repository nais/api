package bucket

import (
	"context"
	"fmt"
	"sort"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

type client struct {
	informers k8s.ClusterInformers
}

func (l client) getBuckets(ctx context.Context, ids []resourceIdentifier) ([]*Bucket, error) {
	ret := make([]*Bucket, 0)
	for _, id := range ids {
		v, err := l.getBucket(ctx, id.environment, id.namespace, id.name)
		if err != nil {
			continue
		}
		ret = append(ret, v)
	}
	return ret, nil
}

func (l client) getBucketsForTeam(_ context.Context, teamSlug slug.Slug) ([]*Bucket, error) {
	ret := make([]*Bucket, 0)

	for env, infs := range l.informers {
		inf := infs.Bucket
		if inf == nil {
			continue
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("listing buckets: %w", err)
		}

		for _, obj := range objs {
			bqs, err := toBucket(obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, fmt.Errorf("converting to buckets: %w", err)
			}

			ret = append(ret, bqs)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

func (l client) getBucket(_ context.Context, env string, namespace string, name string) (*Bucket, error) {
	inf, exists := l.informers[env]
	if !exists {
		return nil, fmt.Errorf("unknown env: %q", env)
	}

	if inf.Bucket == nil {
		return nil, apierror.Errorf("bucket informer not supported in env: %q", env)
	}

	obj, err := inf.Bucket.Lister().ByNamespace(namespace).Get(name)
	if err != nil {
		return nil, fmt.Errorf("get bucket: %w", err)
	}

	return toBucket(obj.(*unstructured.Unstructured), env)
}
