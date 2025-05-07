package price

import "context"

func CPUHour(ctx context.Context) (*Price, error) {
	p, err := fromContext(ctx).client.Price.GetUnitPrice(ctx, "0981-D144-B18E")
	if err != nil {
		return nil, err
	}

	return p, nil
}

func MemoryHour(ctx context.Context) (*Price, error) {
	p, err := fromContext(ctx).client.Price.GetUnitPrice(ctx, "779E-BED5-F31F")
	if err != nil {
		return nil, err
	}

	return p, nil
}
