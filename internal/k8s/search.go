package k8s

import (
	"context"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/search"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Client) Search(ctx context.Context, q string, filter *model.SearchFilter) []*search.Result {
	if !isFilterOrNoFilter(filter) {
		return nil
	}

	if c.database == nil {
		panic("database not set")
	}

	ret := []*search.Result{}

	for env, infs := range c.informers {
		if isFilterSqlInstanceOrNoFilter(filter) {
			if infs.SqlInstanceInformer == nil {
				continue
			}

			sqlInstances, err := infs.SqlInstanceInformer.Lister().List(labels.Everything())
			if err != nil {
				c.error(ctx, err, "listing SQL instances")
				return nil
			}

			for _, obj := range sqlInstances {
				u := obj.(*unstructured.Unstructured)
				rank := search.Match(q, u.GetName())
				if rank == -1 {
					continue
				}

				sqlInstance, err := model.ToSqlInstance(u, env)
				if err != nil {
					c.error(ctx, err, "converting to SQL instance")
					return nil
				} else if ok, _ := c.database.TeamExists(ctx, sqlInstance.GQLVars.TeamSlug); !ok {
					continue
				}

				ret = append(ret, &search.Result{
					Node: sqlInstance,
					Rank: rank,
				})
			}
		}

		if isFilterNaisjobOrNoFilter(filter) {
			jobs, err := infs.NaisjobInformer.Lister().List(labels.Everything())
			if err != nil {
				c.error(ctx, err, "listing jobs")
				return nil
			}

			for _, obj := range jobs {
				u := obj.(*unstructured.Unstructured)
				rank := search.Match(q, u.GetName())
				if rank == -1 {
					continue
				}
				job, err := c.ToNaisJob(u, env)
				if err != nil {
					c.error(ctx, err, "converting to job")
					return nil
				} else if ok, _ := c.database.TeamExists(ctx, job.GQLVars.Team); !ok {
					continue
				}

				ret = append(ret, &search.Result{
					Node: job,
					Rank: rank,
				})
			}
		}

		if isFilterAppOrNoFilter(filter) {
			apps, err := infs.AppInformer.Lister().List(labels.Everything())
			if err != nil {
				c.error(ctx, err, "listing applications")
				return nil
			}

			for _, obj := range apps {
				u := obj.(*unstructured.Unstructured)
				rank := search.Match(q, u.GetName())
				if rank == -1 {
					continue
				}
				app, err := c.toApp(ctx, u, env)
				if err != nil {
					c.error(ctx, err, "converting to app")
					return nil
				} else if ok, _ := c.database.TeamExists(ctx, app.GQLVars.Team); !ok {
					continue
				}

				ret = append(ret, &search.Result{
					Node: app,
					Rank: rank,
				})
			}
		}

	}
	return ret
}

func isFilter(filter *model.SearchFilter) bool {
	if filter == nil {
		return false
	}

	if filter.Type == nil {
		return false
	}

	return true
}

func isFilterOrNoFilter(filter *model.SearchFilter) bool {
	if !isFilter(filter) {
		return true
	}

	return *filter.Type == model.SearchTypeApp || *filter.Type == model.SearchTypeNaisjob || *filter.Type == model.SearchTypeSQLInstance
}

func isFilterAppOrNoFilter(filter *model.SearchFilter) bool {
	if !isFilter(filter) {
		return true
	}

	return *filter.Type == model.SearchTypeApp
}

func isFilterNaisjobOrNoFilter(filter *model.SearchFilter) bool {
	if !isFilter(filter) {
		return true
	}

	return *filter.Type == model.SearchTypeNaisjob
}

func isFilterSqlInstanceOrNoFilter(filter *model.SearchFilter) bool {
	if !isFilter(filter) {
		return true
	}

	return *filter.Type == model.SearchTypeSQLInstance
}
