package price

import (
	"context"

	"github.com/sirupsen/logrus"
)

type Client struct {
	Price *PriceService
	log   logrus.FieldLogger
}

func NewClient(ctx context.Context, log logrus.FieldLogger) (*Client, error) {
	client := &Client{
		log: log,
	}

	s, err := NewPriceService(ctx, log)
	if err != nil {
		return nil, err
	}
	client.Price = s

	return client, nil
}
