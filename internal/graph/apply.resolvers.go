package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/apply"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model/donotuse"
)

func (r *applyActivityLogEntryResolver) Action(ctx context.Context, obj *apply.ApplyActivityLogEntry) (string, error) {
	return string(obj.GenericActivityLogEntry.Action), nil
}

func (r *applyActivityLogEntryDataResolver) ChangedFields(ctx context.Context, obj *apply.ApplyActivityLogEntryData) ([]*donotuse.ApplyChangedField, error) {
	out := make([]*donotuse.ApplyChangedField, len(obj.ChangedFields))
	for i, c := range obj.ChangedFields {
		field := &donotuse.ApplyChangedField{
			Field: c.Field,
		}
		if c.OldValue != nil {
			s := fmt.Sprintf("%v", c.OldValue)
			field.OldValue = &s
		}
		if c.NewValue != nil {
			s := fmt.Sprintf("%v", c.NewValue)
			field.NewValue = &s
		}
		out[i] = field
	}
	return out, nil
}

func (r *Resolver) ApplyActivityLogEntry() gengql.ApplyActivityLogEntryResolver {
	return &applyActivityLogEntryResolver{r}
}

func (r *Resolver) ApplyActivityLogEntryData() gengql.ApplyActivityLogEntryDataResolver {
	return &applyActivityLogEntryDataResolver{r}
}

type (
	applyActivityLogEntryResolver     struct{ *Resolver }
	applyActivityLogEntryDataResolver struct{ *Resolver }
)
