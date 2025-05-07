package price

import (
	"context"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	cloudbilling "google.golang.org/api/cloudbilling/v1beta"
	"google.golang.org/api/option"
)

type PriceService struct {
	cache  *cache.Cache
	client *cloudbilling.Service
	log    logrus.FieldLogger
}

func NewPriceService(ctx context.Context, log logrus.FieldLogger, opts ...option.ClientOption) (*PriceService, error) {
	priceService, err := cloudbilling.NewService(ctx, option.WithScopes(cloudbilling.CloudBillingScope))
	if err != nil {
		log.Fatalf("Failed to create billing service: %v", err)
	}

	return &PriceService{
		cache:  cache.New(10*time.Minute, 20*time.Minute),
		client: priceService,
		log:    log,
	}, nil
}

func (s *PriceService) GetUnitPrice(ctx context.Context, resourceType ResourceType) (*Price, error) {
	// TODO: Replace with actual SKU
	p, err := s.client.Skus.Price.Get("skus/0001-48D2-BE14/price").CurrencyCode("EUR").Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	return &Price{
		Currency:    p.CurrencyCode,
		Description: "Price for " + resourceType.String(),
		Type:        resourceType,
		Price:       float64(p.Rate.Tiers[len(p.Rate.Tiers)-1].ListPrice.Nanos) / 1e9,
		Unit:        "TODO",
	}, nil
}
