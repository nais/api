package price

import (
	"context"
	"fmt"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	cloudbilling "google.golang.org/api/cloudbilling/v1beta"
	"google.golang.org/api/option"
)

type Client struct {
	cache  *cache.Cache
	client *cloudbilling.Service
	log    logrus.FieldLogger
}

type Retriever interface {
	GetUnitPrice(ctx context.Context, skuID string) (*Price, error)
}

func NewClient(ctx context.Context, log logrus.FieldLogger, opts ...option.ClientOption) (*Client, error) {
	priceService, err := cloudbilling.NewService(ctx, option.WithScopes(cloudbilling.CloudBillingScope))
	if err != nil {
		return nil, fmt.Errorf("failed to create billing service: %w", err)
	}

	return &Client{
		cache:  cache.New(10*time.Minute, 20*time.Minute),
		client: priceService,
		log:    log,
	}, nil
}

func (s *Client) GetUnitPrice(ctx context.Context, skuID string) (*Price, error) {
	if cached, found := s.cache.Get(skuID); found {
		if price, ok := cached.(*Price); ok {
			return price, nil
		}
	}

	p, err := s.client.Skus.Price.Get("skus/" + skuID + "/price").CurrencyCode("EUR").Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	price := &Price{
		Value: float64(p.Rate.Tiers[len(p.Rate.Tiers)-1].ListPrice.Nanos) / 1e9,
	}

	s.cache.Add(skuID, price, time.Hour*24)

	return price, nil
}
