package graph

import (
	"reflect"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/persistence/bigquery"
	"github.com/nais/api/internal/persistence/kafkatopic"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/user"
	"github.com/nais/api/internal/vulnerability"
)

func Complexity() gengql.ComplexityRoot {
	c := gengql.ComplexityRoot{}

	c.Application.Instances = stdListComplexity
	c.Application.Secrets = stdListComplexity
	c.ContainerImage.WorkloadReferences = stdListComplexity
	c.DeploymentInfo.History = stdListComplexity
	c.ImageVulnerabilityAnalysisTrail.Comments = stdListComplexity
	c.Job.Runs = stdListComplexity
	c.Job.Secrets = stdListComplexity
	c.JobRun.Instances = stdListComplexity
	c.Query.Reconcilers = stdListComplexity

	c.Query.Users = func(childComplexity int, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *user.UserOrder) int {
		return cursorComplexity(first, last) * childComplexity
	}

	c.ContainerImage.Vulnerabilities = func(childComplexity int, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *vulnerability.ImageVulnerabilityOrder) int {
		return cursorComplexity(first, last) * childComplexity
	}

	c.BigQueryDataset.Access = func(childComplexity int, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *bigquery.BigQueryDatasetAccessOrder) int {
		return cursorComplexity(first, last) * childComplexity
	}

	c.KafkaTopic.ACL = func(childComplexity int, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *kafkatopic.KafkaTopicACLFilter, orderBy *kafkatopic.KafkaTopicACLOrder) int {
		return cursorComplexity(first, last) * childComplexity
	}

	c.OpenSearch.Access = func(childComplexity int, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *opensearch.OpenSearchAccessOrder) int {
		return cursorComplexity(first, last) * childComplexity
	}

	// c.Query.Search =

	return c
}

func cursorComplexity(first, last *int) int {
	if first != nil {
		return *first
	}
	if last != nil {
		return *last
	}
	return 100
}

func stdListComplexity(childComplexity int, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) int {
	return cursorComplexity(first, last) * childComplexity
}

// newComplexity returns a new ComplexityRoot with all cursor based complexity functions
// implemented to (first | last) * childComplexity
func newComplexity() gengql.ComplexityRoot {
	c := gengql.ComplexityRoot{}
	t := reflect.TypeOf(c)
	v := reflect.ValueOf(&c).Elem()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Type.Kind() == reflect.Func {
			v.Field(i).Set(reflect.MakeFunc(field.Type, func(args []reflect.Value) []reflect.Value {
				return []reflect.Value{reflect.ValueOf(cursorComplexity(args[1].Interface().(*int), args[3].Interface().(*int)))}
			}))
		}
	}

	return c
}
