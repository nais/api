package k8s

import (
	"context"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Client) Secrets(ctx context.Context, name, team, env string) ([]*model.Secret, error) {
	ret := make([]*model.Secret, 0)
	relevantSecretLabels := labels.SelectorFromSet(map[string]string{
		"nais.io/managed-by": "console",
	})
	for env, infs := range c.informers {

		objs, err := infs.SecretInformer.Lister().Secrets(team).List(relevantSecretLabels)
		if err != nil {
			return nil, c.error(ctx, err, "listing applications")
		}
		for _, obj := range objs {
			secret := model.Secret{
				Env: model.Env{
					Name: env,
					ID:   scalar.EnvIdent(env),
				},
				Name: obj.Name,
				Data: secretBytesToString(obj.Data),
			}
			ret = append(ret, &secret)
		}

	}
	return ret, nil
}

func secretBytesToString(data map[string][]byte) map[string]string {
	ret := make(map[string]string, 0)
	for key, value := range data {
		ret[key] = string(value)
	}
	return ret
}

func (c *Client) CreateSecret(ctx context.Context, secret *model.Secret) error { return nil }

func (c *Client) UpdateSecret(ctx context.Context, secret *model.Secret) error { return nil }

func (c *Client) DeleteSecret(ctx context.Context, secret *model.Secret) error { return nil }
