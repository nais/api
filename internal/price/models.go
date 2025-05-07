package price

type Price struct {
	Value float64 `json:"value"`
}

// CurrentUnitPrices is an empty placeholder for fields related to unit prices.
// They are not part of the model to force individual resolvers according to the schema.
type CurrentUnitPrices struct{}
