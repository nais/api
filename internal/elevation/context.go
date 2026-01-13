package elevation

import (
	"context"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
)

type ctxKey int

const loadersKey ctxKey = iota

type clients struct {
	k8sClients map[string]dynamic.Interface
	log        logrus.FieldLogger
}

func NewLoaderContext(ctx context.Context, k8sClients map[string]dynamic.Interface, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, &clients{
		k8sClients: k8sClients,
		log:        log,
	})
}

func fromContext(ctx context.Context) *clients {
	return ctx.Value(loadersKey).(*clients)
}

func (c *clients) GetClient(environment string) (dynamic.Interface, bool) {
	client, exists := c.k8sClients[environment]
	return client, exists
}
