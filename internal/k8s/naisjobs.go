package k8s

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	naisv1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	naisv1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	sync_states "github.com/nais/liberator/pkg/events"
	"gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

func (c *Client) DeleteJob(ctx context.Context, name, team, env string) error {
	impersonatedClients, err := c.impersonationClientCreator(ctx)
	if err != nil {
		return c.error(ctx, err, "impersonation")
	}

	cli, ok := impersonatedClients[env]
	if !ok {
		return c.error(ctx, fmt.Errorf("no client set for env %q", env), "getting client")
	}

	app := cli.dynamicClient.Resource(naisv1alpha1.GroupVersion.WithResource("naisjobs")).Namespace(team)
	if err := app.Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return c.error(ctx, err, "deleting naisjob")
	}

	return nil
}
func (c *Client) NaisJob(ctx context.Context, name, team, env string) (*model.NaisJob, error) {
	c.log.Debugf("getting job %q in namespace %q in env %q", name, team, env)
	if c.informers[env] == nil {
		return nil, fmt.Errorf("no jobInformer for env %q", env)
	}
	obj, err := c.informers[env].NaisjobInformer.Lister().ByNamespace(team).Get(name)
	if err != nil {
		return nil, c.error(ctx, err, "getting job")
	}

	job, err := c.ToNaisJob(obj.(*unstructured.Unstructured), env)
	if err != nil {
		return nil, c.error(ctx, err, "converting to job")
	}

	for i, rule := range job.AccessPolicy.Outbound.Rules {
		err = c.setJobHasMutualOnOutbound(ctx, name, team, env, rule)
		if err != nil {
			return nil, c.error(ctx, err, "setting hasMutual on outbound")
		}
		job.AccessPolicy.Outbound.Rules[i] = rule
	}

	for i, rule := range job.AccessPolicy.Inbound.Rules {
		err = c.setJobHasMutualOnInbound(ctx, name, team, env, rule)
		if err != nil {
			return nil, c.error(ctx, err, "setting hasMutual on inbound")
		}
		job.AccessPolicy.Inbound.Rules[i] = rule
	}

	topics, err := c.getTopics(ctx, name, team, env)
	if err != nil {
		return nil, c.error(ctx, err, "getting topics")
	}

	storage, err := naisjobStorage(obj.(*unstructured.Unstructured), topics)
	if err != nil {
		return nil, c.error(ctx, err, "getting storage")
	}

	job.Storage = storage

	runs, err := c.Runs(ctx, team, env, name)
	if err != nil {
		return nil, c.error(ctx, err, "getting runs")
	}

	tmpJob := &naisv1.Naisjob{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(*unstructured.Unstructured).Object, tmpJob); err != nil {
		return nil, fmt.Errorf("converting to application: %w", err)
	}

	setJobStatus(job, *tmpJob.Status.Conditions, runs)

	return job, nil
}

func (c *Client) setJobHasMutualOnOutbound(ctx context.Context, oJob, oTeam, oEnv string, outboundRule *model.Rule) error {
	outboundEnv := oEnv
	if outboundRule.Cluster != "" {
		outboundEnv = outboundRule.Cluster
	}
	outboundTeam := oTeam
	if outboundRule.Namespace != "" {
		outboundTeam = outboundRule.Namespace
	}

	if outboundRule.Application == "*" {
		outboundRule.Mutual = true
		return nil
	}

	noZeroTrust := checkNoZeroTrust(oEnv, outboundRule)
	if noZeroTrust {
		return nil
	}

	inf := c.getInformers(outboundEnv)
	if inf == nil {
		return nil
	}

	app, err := c.getApp(ctx, inf, outboundEnv, outboundTeam, outboundRule.Application)
	if app == nil {
		c.log.Debug("no app found for outbound rule ", outboundRule.Application, " in ", outboundEnv, " for ", outboundTeam, ": ", err)
		outboundRule.Mutual = false
		outboundRule.MutualExplanation = "APP_NOT_FOUND"
		return nil
	}

	for _, inboundRuleOnOutboundApp := range app.AccessPolicy.Inbound.Rules {
		if inboundRuleOnOutboundApp.Cluster != "" {
			if inboundRuleOnOutboundApp.Cluster != "*" && oEnv != inboundRuleOnOutboundApp.Cluster {
				continue
			}
		}

		if inboundRuleOnOutboundApp.Namespace != "" {
			if inboundRuleOnOutboundApp.Namespace != "*" && oTeam != inboundRuleOnOutboundApp.Namespace {
				continue
			}
		}

		if inboundRuleOnOutboundApp.Application == "*" || inboundRuleOnOutboundApp.Application == oJob {
			outboundRule.Mutual = true
			return nil
		}
	}

	outboundRule.Mutual = false
	outboundRule.MutualExplanation = "RULE_NOT_FOUND"

	return nil
}

func (c *Client) setJobHasMutualOnInbound(ctx context.Context, oApp, oTeam, oEnv string, inboundRule *model.Rule) error {
	inboundEnv := oEnv
	if inboundRule.Cluster != "" {
		inboundEnv = inboundRule.Cluster
	}

	inboundTeam := oTeam
	if inboundRule.Namespace != "" {
		inboundTeam = inboundRule.Namespace
	}

	if inboundRule.Application == "*" {
		inboundRule.Mutual = true
		return nil
	}

	noZeroTrust := checkNoZeroTrust(oEnv, inboundRule)
	if noZeroTrust {
		return nil
	}

	inf := c.getInformers(inboundEnv)
	if inf == nil {
		return nil
	}

	app, err := c.getApp(ctx, inf, inboundEnv, inboundTeam, inboundRule.Application)
	if app == nil {
		c.log.Debug("no app found for inbound rule ", inboundRule.Application, " in ", inboundEnv, " for ", inboundTeam, ": ", err)
		inboundRule.Mutual = false
		inboundRule.MutualExplanation = "APP_NOT_FOUND"
		return nil
	}

	for _, outboundRuleOnInboundApp := range app.AccessPolicy.Outbound.Rules {
		if outboundRuleOnInboundApp.Cluster != "" {
			if outboundRuleOnInboundApp.Cluster != "*" && oEnv != outboundRuleOnInboundApp.Cluster {
				continue
			}
		}

		if outboundRuleOnInboundApp.Namespace != "" {
			if outboundRuleOnInboundApp.Namespace != "*" && oTeam != outboundRuleOnInboundApp.Namespace {
				continue
			}
		}

		if outboundRuleOnInboundApp.Application == "*" || outboundRuleOnInboundApp.Application == oApp {
			inboundRule.Mutual = true
			return nil
		}
	}

	inboundRule.Mutual = false
	inboundRule.MutualExplanation = "RULE_NOT_FOUND"
	return nil
}

func setJobStatus(job *model.NaisJob, conditions []metav1.Condition, runs []*model.Run) {
	currentCondition := synchronizationStateCondition(conditions)
	jobState := model.JobState{
		State:  model.StateNais,
		Errors: []model.StateError{},
	}

	if currentCondition != nil {
		switch currentCondition.Reason {
		case sync_states.FailedPrepare:
			jobState.Errors = append(jobState.Errors, &model.InvalidNaisYamlError{
				Revision: job.DeployInfo.CommitSha,
				Level:    model.ErrorLevelWarning,
				Detail:   currentCondition.Message,
			})
			jobState.State = model.StateNotnais
		case sync_states.Retrying:
			fallthrough
		case sync_states.FailedSynchronization:
			jobState.Errors = append(jobState.Errors, &model.SynchronizationFailingError{
				Revision: job.DeployInfo.CommitSha,
				Level:    model.ErrorLevelError,
				Detail:   currentCondition.Message,
			})
			jobState.State = model.StateNotnais
		}
	}

	var tmpTime time.Time
	var tmpRun *model.Run
	for _, run := range runs {
		if run.StartTime != nil && run.StartTime.After(tmpTime) {
			tmpTime = *run.StartTime
			tmpRun = run
		} else {
			continue
		}
	}

	if tmpRun != nil {
		if tmpRun.Failed {
			jobState.Errors = append(jobState.Errors, &model.FailedRunError{
				Revision:   job.DeployInfo.CommitSha,
				Level:      model.ErrorLevelWarning,
				RunMessage: tmpRun.Message,
				RunName:    tmpRun.Name,
			})
			jobState.State = model.StateFailing
		}
	}

	if !strings.Contains(job.Image, "europe-north1-docker.pkg.dev") {
		parts := strings.Split(job.Image, ":")
		tag := "unknown"
		if len(parts) > 1 {
			tag = parts[1]
		}
		parts = strings.Split(parts[0], "/")
		registry := parts[0]
		name := parts[len(parts)-1]
		repository := ""
		if len(parts) > 2 {
			repository = strings.Join(parts[1:len(parts)-1], "/")
		} else {
			repository = "confusus"
		}
		jobState.Errors = append(jobState.Errors, &model.DeprecatedRegistryError{
			Revision:   job.DeployInfo.CommitSha,
			Level:      model.ErrorLevelTodo,
			Registry:   registry,
			Name:       name,
			Tag:        tag,
			Repository: repository,
		})
		/*if jobState.State != model.StateFailing {
			jobState.State = model.StateNotnais
		}*/
	}

	for _, rule := range job.AccessPolicy.Inbound.Rules {
		if !rule.Mutual {
			jobState.Errors = append(jobState.Errors, &model.InboundAccessError{
				Revision: job.DeployInfo.CommitSha,
				Level:    model.ErrorLevelWarning,
				Rule:     *rule,
			})
			if jobState.State != model.StateFailing {
				jobState.State = model.StateNotnais
			}
		}
	}

	for _, rule := range job.AccessPolicy.Outbound.Rules {
		if !rule.Mutual {
			jobState.Errors = append(jobState.Errors, &model.OutboundAccessError{
				Revision: job.DeployInfo.CommitSha,
				Level:    model.ErrorLevelWarning,
				Rule:     *rule,
			})
			if jobState.State != model.StateFailing {
				jobState.State = model.StateNotnais
			}
		}
	}

	job.JobState = jobState
}

func (c *Client) NaisJobs(ctx context.Context, team string) ([]*model.NaisJob, error) {
	ret := make([]*model.NaisJob, 0)

	for env, infs := range c.informers {
		objs, err := infs.NaisjobInformer.Lister().ByNamespace(team).List(labels.Everything())
		if err != nil {
			return nil, c.error(ctx, err, "listing jobs")
		}
		for _, obj := range objs {
			job, err := c.ToNaisJob(obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, c.error(ctx, err, "converting to job")
			}

			for i, rule := range job.AccessPolicy.Outbound.Rules {
				err = c.setJobHasMutualOnOutbound(ctx, job.Name, team, env, rule)
				if err != nil {
					return nil, c.error(ctx, err, "setting hasMutual on outbound")
				}
				job.AccessPolicy.Outbound.Rules[i] = rule
			}

			for i, rule := range job.AccessPolicy.Inbound.Rules {
				err = c.setJobHasMutualOnInbound(ctx, job.Name, team, env, rule)
				if err != nil {
					return nil, c.error(ctx, err, "setting hasMutual on inbound")
				}
				job.AccessPolicy.Inbound.Rules[i] = rule
			}

			runs, err := c.Runs(ctx, team, env, job.Name)
			if err != nil {
				return nil, c.error(ctx, err, "getting runs")
			}

			tmpJob := &naisv1.Naisjob{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(*unstructured.Unstructured).Object, tmpJob); err != nil {
				return nil, fmt.Errorf("converting to naisjob: %w", err)
			}

			setJobStatus(job, *tmpJob.Status.Conditions, runs)

			ret = append(ret, job)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

func (c *Client) NaisJobManifest(ctx context.Context, name, team, env string) (string, error) {
	obj, err := c.informers[env].NaisjobInformer.Lister().ByNamespace(team).Get(name)
	if err != nil {
		return "", c.error(ctx, err, "getting job")
	}
	u := obj.(*unstructured.Unstructured)

	tmp := map[string]any{}

	spec, _, err := unstructured.NestedMap(u.Object, "spec")
	if err != nil {
		return "", c.error(ctx, err, "getting spec")
	}

	tmp["spec"] = spec
	tmp["apiVersion"] = u.GetAPIVersion()
	tmp["kind"] = u.GetKind()
	metadata := map[string]any{"labels": u.GetLabels()}
	metadata["name"] = u.GetName()
	metadata["namespace"] = u.GetNamespace()
	tmp["metadata"] = metadata
	b, err := yaml.Marshal(tmp)
	if err != nil {
		return "", c.error(ctx, err, "marshalling manifest")
	}

	return string(b), nil
}

func (c *Client) Runs(ctx context.Context, team, env, name string) ([]*model.Run, error) {
	ret := make([]*model.Run, 0)
	nameReq, err := labels.NewRequirement("app", selection.Equals, []string{name})
	if err != nil {
		return nil, c.error(ctx, err, "creating label selector")
	}

	selector := labels.NewSelector().Add(*nameReq)

	jobs, err := c.informers[env].JobInformer.Lister().Jobs(team).List(selector)
	if err != nil {
		return nil, c.error(ctx, err, "listing job instances")
	}

	for _, job := range jobs {
		var startTime, completionTime *time.Time
		if job.Status.CompletionTime != nil {
			completionTime = &job.Status.CompletionTime.Time
		}
		if job.Status.StartTime != nil {
			startTime = &job.Status.StartTime.Time
		}

		podReq, err := labels.NewRequirement("job-name", selection.Equals, []string{job.Name})
		if err != nil {
			return nil, c.error(ctx, err, "creating label selector")
		}
		podSelector := labels.NewSelector().Add(*podReq)
		pods, err := c.informers[env].PodInformer.Lister().Pods(team).List(podSelector)
		if err != nil {
			return nil, c.error(ctx, err, "listing job instance pods")
		}

		var podNames []string
		for _, pod := range pods {
			podNames = append(podNames, pod.Name)
		}

		ret = append(ret, &model.Run{
			ID:             scalar.JobIdent(job.Name),
			Name:           job.Name,
			PodNames:       podNames,
			StartTime:      startTime,
			CompletionTime: completionTime,
			Failed:         failed(job),
			Duration:       duration(job).String(),
			Image:          job.Spec.Template.Spec.Containers[0].Image,
			Message:        Message(job),
			GQLVars: model.RunGQLVars{
				Env:     env,
				Team:    slug.Slug(team),
				NaisJob: name,
			},
		})
	}

	sort.Slice(ret, func(i, j int) bool {
		if ret[i].StartTime == nil {
			return false
		}
		if ret[j].StartTime == nil {
			return true
		}

		return ret[i].StartTime.After(*ret[j].StartTime)
	})

	return ret, nil
}

func Message(job *batchv1.Job) string {
	if failed(job) {
		return fmt.Sprintf("Run failed after %d attempts", job.Status.Failed)
	}
	target := completionTarget(*job)
	if job.Status.Active > 0 {
		msg := ""
		if job.Status.Active == 1 {
			msg = "1 instance running"
		} else {
			msg = fmt.Sprintf("%d instances running", job.Status.Active)
		}
		return fmt.Sprintf("%s. %d/%d completed (%d failed %s)", msg, job.Status.Succeeded, target, job.Status.Failed, pluralize("attempt", job.Status.Failed))
	} else if job.Status.Succeeded == target {
		return fmt.Sprintf("%d/%d instances completed (%d failed %s)", job.Status.Succeeded, target, job.Status.Failed, pluralize("attempt", job.Status.Failed))
	}
	return ""
}

func pluralize(s string, count int32) string {
	if count == 1 {
		return s
	}
	return s + "s"
}

// completion target is the number of successful runs we want to see based on parallelism and completions
func completionTarget(job batchv1.Job) int32 {
	if job.Spec.Completions == nil && job.Spec.Parallelism == nil {
		return 1
	}
	if job.Spec.Completions != nil {
		return *job.Spec.Completions
	}
	return *job.Spec.Parallelism
}

func duration(job *batchv1.Job) time.Duration {
	if job.Status.StartTime == nil {
		return time.Duration(0)
	}
	if job.Status.CompletionTime != nil {
		return job.Status.CompletionTime.Sub(job.Status.StartTime.Time)
	}
	if !failed(job) {
		return time.Since(job.Status.StartTime.Time)
	}
	for _, cs := range job.Status.Conditions {
		if cs.Status == corev1.ConditionTrue {
			if cs.Type == batchv1.JobFailed {
				return cs.LastTransitionTime.Time.Sub(job.Status.StartTime.Time)
			}
		}
	}

	return time.Duration(0)
}

func failed(job *batchv1.Job) bool {
	for _, cs := range job.Status.Conditions {
		if cs.Status == corev1.ConditionTrue && cs.Type == batchv1.JobFailed {
			return true
		}
	}
	return false
}

func (c *Client) ToNaisJob(u *unstructured.Unstructured, env string) (*model.NaisJob, error) {
	naisjob := &naisv1.Naisjob{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, naisjob); err != nil {
		return nil, fmt.Errorf("converting to job: %w", err)
	}

	ret := &model.NaisJob{}
	ret.ID = scalar.JobIdent("job_" + env + "_" + naisjob.GetNamespace() + "_" + naisjob.GetName())
	ret.Name = naisjob.GetName()
	ret.Env = model.Env{
		Team: naisjob.GetNamespace(),
		Name: env,
	}

	ret.DeployInfo = model.DeployInfo{
		CommitSha: naisjob.GetAnnotations()["deploy.nais.io/github-sha"],
		Deployer:  naisjob.GetAnnotations()["deploy.nais.io/github-actor"],
		URL:       naisjob.GetAnnotations()["deploy.nais.io/github-workflow-run-url"],
	}
	ret.DeployInfo.GQLVars.Job = naisjob.GetName()
	ret.DeployInfo.GQLVars.Env = env
	ret.DeployInfo.GQLVars.Team = slug.Slug(naisjob.GetNamespace())

	timestamp := time.Unix(0, naisjob.GetStatus().RolloutCompleteTime)
	ret.DeployInfo.Timestamp = &timestamp
	ret.GQLVars.Team = slug.Slug(naisjob.GetNamespace())
	ret.Image = naisjob.Spec.Image

	ap := model.AccessPolicy{}
	if err := convert(naisjob.Spec.AccessPolicy, &ap); err != nil {
		return nil, fmt.Errorf("converting accessPolicy: %w", err)
	}
	ret.AccessPolicy = ap

	r := model.Resources{}
	if err := convert(naisjob.Spec.Resources, &r); err != nil {
		return nil, fmt.Errorf("converting resources: %w", err)
	}

	r.Requests = model.Requests{}
	r.Limits = model.Limits{}
	ret.Resources = r

	ret.Schedule = naisjob.Spec.Schedule

	if naisjob.Spec.Completions != nil {
		ret.Completions = int(*naisjob.Spec.Completions)
	}
	if naisjob.Spec.Parallelism != nil {
		ret.Parallelism = int(*naisjob.Spec.Parallelism)
	}
	ret.Retries = int(naisjob.Spec.BackoffLimit)

	authz, err := jobAuthz(naisjob)
	if err != nil {
		return nil, fmt.Errorf("getting authz: %w", err)
	}

	ret.Authz = authz

	secrets := make([]string, 0)
	for _, filesFrom := range naisjob.Spec.FilesFrom {
		secrets = append(secrets, filesFrom.Secret)
	}
	for _, secretName := range naisjob.Spec.EnvFrom {
		secrets = append(secrets, secretName.Secret)
	}

	slices.Sort(secrets)
	ret.GQLVars.SecretNames = slices.Compact(secrets)

	return ret, nil
}

func naisjobStorage(u *unstructured.Unstructured, topics []*model.Topic) ([]model.Storage, error) {
	naisjob := &naisv1.Naisjob{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, naisjob); err != nil {
		return nil, fmt.Errorf("converting to application: %w", err)
	}

	ret := make([]model.Storage, 0)

	if naisjob.Spec.GCP != nil {
		for _, v := range naisjob.Spec.GCP.Buckets {
			bucket := model.Bucket{}
			if err := convert(v, &bucket); err != nil {
				return nil, fmt.Errorf("converting buckets: %w", err)
			}
			ret = append(ret, bucket)
		}
		for _, v := range naisjob.Spec.GCP.SqlInstances {
			sqlInstance := model.SQLInstance{}
			if err := convert(v, &sqlInstance); err != nil {
				return nil, fmt.Errorf("converting sqlInstance: %w", err)
			}
			if sqlInstance.Name == "" {
				sqlInstance.Name = naisjob.Name
			}
			ret = append(ret, sqlInstance)
		}

		for _, v := range naisjob.Spec.GCP.BigQueryDatasets {
			bqDataset := model.BigQueryDataset{}
			if err := convert(v, &bqDataset); err != nil {
				return nil, fmt.Errorf("converting bigQueryDataset: %w", err)
			}
			ret = append(ret, bqDataset)
		}
	}

	if naisjob.Spec.OpenSearch != nil {
		os := model.OpenSearch{
			Name:   naisjob.Spec.OpenSearch.Instance,
			Access: naisjob.Spec.OpenSearch.Access,
		}
		ret = append(ret, os)
	}

	if naisjob.Spec.Kafka != nil {
		kafka := model.Kafka{
			Name:    naisjob.Spec.Kafka.Pool,
			Streams: naisjob.Spec.Kafka.Streams,
			Topics:  topics,
		}
		ret = append(ret, kafka)
	}

	if len(naisjob.Spec.Redis) > 0 {
		for _, v := range naisjob.Spec.Redis {
			redis := model.Redis{
				Name:   v.Instance,
				Access: v.Access,
			}
			ret = append(ret, redis)
		}
	}

	if naisjob.Spec.Influx != nil {
		influx := model.InfluxDb{
			Name: naisjob.Spec.Influx.Instance,
		}
		ret = append(ret, influx)
	}
	return ret, nil
}

func jobAuthz(job *naisv1.Naisjob) ([]model.Authz, error) {
	ret := make([]model.Authz, 0)
	if job.Spec.Azure != nil {
		isApp := job.Spec.Azure.Application != nil && job.Spec.Azure.Application.Enabled
		if isApp {
			azureAd := model.AzureAd{}
			if err := convert(job.Spec.Azure, &azureAd); err != nil {
				return nil, fmt.Errorf("converting azureAd: %w", err)
			}
			ret = append(ret, azureAd)
		}
	}

	if job.Spec.Maskinporten != nil && job.Spec.Maskinporten.Enabled {
		maskinporten := model.Maskinporten{}
		if err := convert(job.Spec.Maskinporten, &maskinporten); err != nil {
			return nil, fmt.Errorf("converting maskinporten: %w", err)
		}
		ret = append(ret, maskinporten)
	}

	return ret, nil
}
