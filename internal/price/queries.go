package price

import "context"

func CPUHour(ctx context.Context) (*Price, error) {
	// 0981-D144-B18E: E2 Instance Core running in Finland
	p, err := fromContext(ctx).client.GetUnitPrice(ctx, "0981-D144-B18E")
	if err != nil {
		return nil, err
	}

	return p, nil
}

func MemoryHour(ctx context.Context) (*Price, error) {
	// 779E-BED5-F31F: E2 Instance Ram running in Finland
	p, err := fromContext(ctx).client.GetUnitPrice(ctx, "779E-BED5-F31F")
	if err != nil {
		return nil, err
	}

	return p, nil
}
