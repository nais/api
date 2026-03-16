package aivencredentials

import (
	"context"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
)

type ctxKey int

const clientsKey ctxKey = iota

func NewClientContext(ctx context.Context, dynamicClients map[string]dynamic.Interface, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, clientsKey, &clients{
		dynamicClients: dynamicClients,
		log:            log,
	})
}

type clients struct {
	dynamicClients map[string]dynamic.Interface
	log            logrus.FieldLogger
}

func fromContext(ctx context.Context) *clients {
	return ctx.Value(clientsKey).(*clients)
}
