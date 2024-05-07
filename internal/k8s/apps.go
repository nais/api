package k8s

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	naisv1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	sync_states "github.com/nais/liberator/pkg/events"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

func getDeprecatedIngresses(cluster string) []string {
	deprecatedIngresses := map[string][]string{
		"dev-fss": {
			"adeo.no",
			"intern.dev.adeo.no",
			"dev-fss.nais.io",
			"dev.adeo.no",
			"dev.intern.nav.no",
			"nais.preprod.local",
		},
		"dev-gcp": {
			"dev-gcp.nais.io",
			"dev.intern.nav.no",
			"dev.nav.no",
			"intern.nav.no",
			"dev.adeo.no",
			"labs.nais.io",
			"ekstern.dev.nais.io",
		},
		"prod-fss": {
			"adeo.no",
			"nais.adeo.no",
			"prod-fss.nais.io",
		},
		"prod-gcp": {
			"dev.intern.nav.no",
			"prod-gcp.nais.io",
		},
	}
	ingresses, ok := deprecatedIngresses[cluster]
	if !ok {
		return []string{}
	}
	return ingresses
}

// AppExists returns true if the given app exists in the given environment. The app informer should be synced before
// calling this function.
func (c *Client) AppExists(env, team, app string) bool {
	if c.informers[env] == nil {
		return false
	}

	_, err := c.informers[env].AppInformer.Lister().ByNamespace(team).Get(app)
	return err == nil
}

func (c *Client) App(ctx context.Context, name, team, env string) (*model.App, error) {
	if c.informers[env] == nil {
		return nil, fmt.Errorf("no appInformer for env %q", env)
	}

	c.log.Debugf("getting app %q in namespace %q in env %q", name, team, env)
	obj, err := c.informers[env].AppInformer.Lister().ByNamespace(team).Get(name)
	if err != nil {
		return nil, c.error(ctx, err, "getting application "+name+"."+team+"."+env)
	}

	app, err := c.toApp(ctx, obj.(*unstructured.Unstructured), env)
	if err != nil {
		return nil, c.error(ctx, err, "converting to app")
	}

	for i, rule := range app.AccessPolicy.Outbound.Rules {
		err = c.setHasMutualOnOutbound(ctx, name, team, env, rule)
		if err != nil {
			return nil, c.error(ctx, err, "setting hasMutual on outbound")
		}
		app.AccessPolicy.Outbound.Rules[i] = rule
	}

	for i, rule := range app.AccessPolicy.Inbound.Rules {
		err = c.setHasMutualOnInbound(ctx, name, team, env, rule)
		if err != nil {
			return nil, c.error(ctx, err, "setting hasMutual on inbound")
		}
		app.AccessPolicy.Inbound.Rules[i] = rule
	}

	topics, err := c.getTopics(ctx, name, team, env)
	if err != nil {
		return nil, c.error(ctx, err, "getting topics")
	}

	storage, err := c.appPersistence(obj.(*unstructured.Unstructured), topics, env)
	if err != nil {
		return nil, c.error(ctx, err, "converting to app storage")
	}

	app.Persistence = storage

	instances, err := c.Instances(ctx, team, env, name)
	if err != nil {
		return nil, c.error(ctx, err, "getting instances")
	}

	tmpApp := &naisv1alpha1.Application{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(*unstructured.Unstructured).Object, tmpApp); err != nil {
		return nil, fmt.Errorf("converting to application: %w", err)
	}

	conditions := tmpApp.Status.Conditions
	if conditions == nil {
		conditions = &[]metav1.Condition{}
	}

	setStatus(app, *conditions, instances)

	return app, nil
}

func (c *Client) setHasMutualOnOutbound(ctx context.Context, oApp, oTeam, oEnv string, outboundRule *model.Rule) error {
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
	if err != nil {
		c.log.Debug("no app found for inbound rule ", outboundRule.Application, " in ", outboundEnv, " for ", outboundTeam, ": ", err)
	}

	naisjob, err := c.getNaisJob(ctx, inf, outboundEnv, outboundTeam, outboundRule.Application)
	if err != nil {
		c.log.Debug("no job found for inbound rule ", outboundRule.Application, " in ", outboundEnv, " for ", outboundTeam, ": ", err)
	}

	if naisjob == nil && app == nil {
		c.log.Debug("no app found for inbound rule ", outboundRule.Application, " in ", outboundEnv, " for ", outboundTeam, ": ", err)
		outboundRule.Mutual = false
		outboundRule.MutualExplanation = "APP_NOT_FOUND"
		return nil
	}

	if app != nil {
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

			if inboundRuleOnOutboundApp.Application == "*" || inboundRuleOnOutboundApp.Application == oApp {
				outboundRule.Mutual = true
				return nil
			}
		}
	}

	if naisjob != nil {
		for _, inboundRuleOnOutboundJob := range naisjob.AccessPolicy.Inbound.Rules {
			if inboundRuleOnOutboundJob.Cluster != "" {
				if inboundRuleOnOutboundJob.Cluster != "*" && oEnv != inboundRuleOnOutboundJob.Cluster {
					continue
				}
			}

			if inboundRuleOnOutboundJob.Namespace != "" {
				if inboundRuleOnOutboundJob.Namespace != "*" && oTeam != inboundRuleOnOutboundJob.Namespace {
					continue
				}
			}

			if inboundRuleOnOutboundJob.Application == "*" || inboundRuleOnOutboundJob.Application == oApp {

				outboundRule.Mutual = true
				outboundRule.IsJob = true
				return nil
			}
		}
	}

	outboundRule.Mutual = false
	outboundRule.MutualExplanation = "RULE_NOT_FOUND"

	return nil
}

func (c *Client) setHasMutualOnInbound(ctx context.Context, oApp, oTeam, oEnv string, inboundRule *model.Rule) error {
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
	} else if strings.EqualFold(inboundRule.Application, "localhost") {
		inboundRule.Mutual = true
		inboundRule.MutualExplanation = "LOCALHOST"
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
	if err != nil {
		c.log.Debug("no app found for inbound rule ", inboundRule.Application, " in ", inboundEnv, " for ", inboundTeam, ": ", err)
	}

	naisjob, err := c.getNaisJob(ctx, inf, inboundEnv, inboundTeam, inboundRule.Application)
	if err != nil {
		c.log.Debug("no job found for inbound rule ", inboundRule.Application, " in ", inboundEnv, " for ", inboundTeam, ": ", err)
	}

	if naisjob == nil && app == nil {
		c.log.Debug("no app found for inbound rule ", inboundRule.Application, " in ", inboundEnv, " for ", inboundTeam, ": ", err)
		inboundRule.Mutual = false
		inboundRule.MutualExplanation = "APP_NOT_FOUND"
		return nil
	}
	if app != nil {
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
	}

	if naisjob != nil {
		for _, outboundRuleOnInboundJob := range naisjob.AccessPolicy.Outbound.Rules {
			if outboundRuleOnInboundJob.Cluster != "" {
				if outboundRuleOnInboundJob.Cluster != "*" && oEnv != outboundRuleOnInboundJob.Cluster {
					continue
				}
			}

			if outboundRuleOnInboundJob.Namespace != "" {
				if outboundRuleOnInboundJob.Namespace != "*" && oTeam != outboundRuleOnInboundJob.Namespace {
					continue
				}
			}

			if outboundRuleOnInboundJob.Application == "*" || outboundRuleOnInboundJob.Application == oApp {
				inboundRule.Mutual = true
				inboundRule.IsJob = true
				return nil
			}
		}
	}

	inboundRule.Mutual = false
	inboundRule.MutualExplanation = "RULE_NOT_FOUND"
	return nil
}

func (c *Client) getInformers(outboundEnv string) *Informers {
	inf, ok := c.informers[outboundEnv]
	if !ok {
		c.log.Warn("no informers for cluster ", outboundEnv)
		return nil
	}

	return inf
}

func checkNoZeroTrust(env string, rule *model.Rule) bool {
	if strings.Contains(env, "-fss") {
		rule.MutualExplanation = "NO_ZERO_TRUST"
		rule.Mutual = true
		return true
	}

	if strings.Contains(rule.Cluster, "-fss") {
		rule.MutualExplanation = "NO_ZERO_TRUST"
		rule.Mutual = true
		return true
	}

	if rule.Namespace == "nais-system" {
		rule.MutualExplanation = "NO_ZERO_TRUST"
		rule.Mutual = true
		return true
	}

	if strings.Contains(rule.Cluster, "-external") {
		rule.MutualExplanation = "NO_ZERO_TRUST"
		rule.Mutual = true
		return true
	}

	return false
}

func (c *Client) getApp(ctx context.Context, inf *Informers, env string, team string, app string) (*model.App, error) {
	obj, err := inf.AppInformer.Lister().ByNamespace(team).Get(app)
	if err != nil {
		if notFoundError(err) {
			return nil, nil
		}
		return nil, err
	}

	application, err := c.toApp(ctx, obj.(*unstructured.Unstructured), env)
	if err != nil {
		return nil, c.error(ctx, err, "converting to app")
	}
	return application, nil
}

func (c *Client) getNaisJob(ctx context.Context, inf *Informers, env, team, job string) (*model.NaisJob, error) {
	obj, err := inf.NaisjobInformer.Lister().ByNamespace(team).Get(job)
	if err != nil {
		if notFoundError(err) {
			return nil, nil
		}
		return nil, err
	}

	naisjob, err := c.ToNaisJob(obj.(*unstructured.Unstructured), env)
	if err != nil {
		return nil, c.error(ctx, err, "converting to naisjob")
	}
	return naisjob, nil
}

func (c *Client) getTopics(ctx context.Context, name, team, env string) ([]*model.Topic, error) {
	// HACK: dev-fss and prod-fss have topic resources in dev-gcp and prod-gcp respectively.
	topicEnv := env
	if env == "dev-fss" {
		topicEnv = "dev-gcp"
	}
	if env == "prod-fss" {
		topicEnv = "prod-gcp"
	}

	if c.informers[topicEnv].TopicInformer == nil {
		return []*model.Topic{}, nil
	}

	topics, err := c.informers[topicEnv].TopicInformer.Lister().List(labels.Everything())
	if err != nil {
		return nil, c.error(ctx, err, "listing topics")
	}

	ret := make([]*model.Topic, 0)
	for _, topic := range topics {
		u := topic.(*unstructured.Unstructured)
		t, err := toTopic(u, name, team)
		if err != nil {
			return nil, c.error(ctx, err, "converting to topic")
		}

		for _, acl := range t.ACL {
			if acl.Team.String() == team && acl.Application == name {
				ret = append(ret, t)
			}
		}
	}

	return ret, nil
}

func (c *Client) Manifest(ctx context.Context, name, team, env string) (string, error) {
	obj, err := c.informers[env].AppInformer.Lister().ByNamespace(team).Get(name)
	if err != nil {
		return "", c.error(ctx, err, "getting application "+name+"."+team+"."+env)
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

type EnvFilter = func(string) bool

func WithEnvs(envs ...string) EnvFilter {
	return func(env string) bool {
		for _, e := range envs {
			if e == env {
				return true
			}
		}
		return false
	}
}

func filterEnvs(env string, filters ...EnvFilter) bool {
	found := 0
	for _, f := range filters {
		if f(env) {
			found++
		}
	}
	return found == len(filters)
}

func (c *Client) Apps(ctx context.Context, team string, filter ...EnvFilter) ([]*model.App, error) {
	ret := make([]*model.App, 0)

	for env, infs := range c.informers {
		if !filterEnvs(env, filter...) {
			continue
		}

		objs, err := infs.AppInformer.Lister().ByNamespace(team).List(labels.Everything())
		if err != nil {
			return nil, c.error(ctx, err, "listing applications")
		}

		for _, obj := range objs {
			app, err := c.toApp(ctx, obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, c.error(ctx, err, "converting to app")
			}

			for i, rule := range app.AccessPolicy.Outbound.Rules {
				err = c.setHasMutualOnOutbound(ctx, app.Name, team, env, rule)
				if err != nil {
					return nil, c.error(ctx, err, "setting hasMutual on outbound")
				}
				app.AccessPolicy.Outbound.Rules[i] = rule
			}

			for i, rule := range app.AccessPolicy.Inbound.Rules {
				err = c.setHasMutualOnInbound(ctx, app.Name, team, env, rule)
				if err != nil {
					return nil, c.error(ctx, err, "setting hasMutual on inbound")
				}
				app.AccessPolicy.Inbound.Rules[i] = rule
			}

			instances, err := c.Instances(ctx, team, env, app.Name)
			if err != nil {
				return nil, c.error(ctx, err, "getting instances")
			}

			tmpApp := &naisv1alpha1.Application{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(*unstructured.Unstructured).Object, tmpApp); err != nil {
				return nil, fmt.Errorf("converting to application: %w", err)
			}

			setStatus(app, ptr.Deref(tmpApp.Status.Conditions, nil), instances)
			ret = append(ret, app)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		if ret[i].Name == ret[j].Name {
			return ret[i].Env.Name < ret[j].Env.Name
		}
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

func (c *Client) Instances(ctx context.Context, team, env, name string) ([]*model.Instance, error) {
	req, err := labels.NewRequirement("app", selection.Equals, []string{name})
	if err != nil {
		return nil, c.error(ctx, err, "creating label selector")
	}

	selector := labels.NewSelector().Add(*req)
	pods, err := c.informers[env].PodInformer.Lister().Pods(team).List(selector)
	if err != nil {
		return nil, c.error(ctx, err, "listing pods")
	}

	ret := make([]*model.Instance, 0)
	for _, pod := range pods {
		instance := Instance(pod, env)
		ret = append(ret, instance)
	}
	return ret, nil
}

func Instance(pod *corev1.Pod, env string) *model.Instance {
	appName := pod.Labels["app"]

	image := "unknown"
	for _, c := range pod.Spec.Containers {
		if c.Name == appName {
			image = c.Image
		}
	}

	appCS := appContainerStatus(pod, appName)
	restarts := 0
	if appCS != nil {
		restarts = int(appCS.RestartCount)
	}

	ret := &model.Instance{
		ID:       scalar.PodIdent(pod.GetUID()),
		Name:     pod.GetName(),
		Image:    image,
		Restarts: restarts,
		Message:  messageFromCS(appCS),
		State:    stateFromCS(appCS),
		Created:  pod.GetCreationTimestamp().Time,
		GQLVars: model.InstanceGQLVars{
			Env:     env,
			Team:    slug.Slug(pod.GetNamespace()),
			AppName: appName,
		},
	}

	return ret
}

func stateFromCS(cs *corev1.ContainerStatus) model.InstanceState {
	switch {
	case cs == nil:
		return model.InstanceStateUnknown
	case cs.State.Running != nil:
		return model.InstanceStateRunning
	case cs.State.Waiting != nil:
		return model.InstanceStateFailing
	default:
		return model.InstanceStateUnknown
	}
}

func messageFromCS(cs *corev1.ContainerStatus) string {
	if cs == nil {
		return ""
	}

	if cs.State.Waiting != nil {
		switch cs.State.Waiting.Reason {
		case "CrashLoopBackOff":
			return "Process is crashing, check logs"
		case "ErrImagePull", "ImagePullBackOff":
			return "Unable to pull image"
		case "CreateContainerConfigError":
			return "Invalid instance configuration, check logs"
		}
	}

	return ""
}

func appContainerStatus(pod *corev1.Pod, appName string) *corev1.ContainerStatus {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == appName {
			return &cs
		}
	}
	return nil
}

func (c *Client) toApp(_ context.Context, u *unstructured.Unstructured, env string) (*model.App, error) {
	app := &naisv1alpha1.Application{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, app); err != nil {
		return nil, fmt.Errorf("converting to application: %w", err)
	}

	ret := &model.App{}
	ret.ID = scalar.AppIdent("app_" + env + "_" + app.GetNamespace() + "_" + app.GetName())
	ret.Name = app.GetName()

	ret.Env = model.Env{
		Team: app.GetNamespace(),
		Name: env,
	}

	appSynchState := app.GetStatus().SynchronizationState

	switch appSynchState {
	case sync_states.RolloutComplete:
		timestamp := time.Unix(0, app.GetStatus().RolloutCompleteTime)
		ret.DeployInfo.Timestamp = &timestamp
	case sync_states.Synchronized:
		timestamp := time.Unix(0, app.GetStatus().SynchronizationTime)
		ret.DeployInfo.Timestamp = &timestamp
	default:
		ret.DeployInfo.Timestamp = nil
	}

	ret.DeployInfo.CommitSha = app.GetAnnotations()["deploy.nais.io/github-sha"]
	ret.DeployInfo.Deployer = app.GetAnnotations()["deploy.nais.io/github-actor"]
	ret.DeployInfo.URL = app.GetAnnotations()["deploy.nais.io/github-workflow-run-url"]
	ret.DeployInfo.GQLVars.App = app.GetName()
	ret.DeployInfo.GQLVars.Env = env
	ret.DeployInfo.GQLVars.Team = slug.Slug(app.GetNamespace())
	ret.GQLVars.Team = slug.Slug(app.GetNamespace())

	ret.Image = app.Spec.Image

	ingresses := make([]string, 0)
	if err := convert(app.Spec.Ingresses, &ingresses); err != nil {
		return nil, fmt.Errorf("converting ingresses: %w", err)
	}
	ret.Ingresses = ingresses

	if app.Spec.Replicas != nil {
		//lint:ignore SA1019 we need to fallback to this value
		cpuThreshold := app.Spec.Replicas.CpuThresholdPercentage
		if app.Spec.Replicas.ScalingStrategy != nil && app.Spec.Replicas.ScalingStrategy.Cpu != nil && app.Spec.Replicas.ScalingStrategy.Cpu.ThresholdPercentage > 0 {
			cpuThreshold = app.Spec.Replicas.ScalingStrategy.Cpu.ThresholdPercentage
		}
		ret.AutoScaling = model.AutoScaling{
			Disabled:     app.Spec.Replicas.DisableAutoScaling,
			CPUThreshold: cpuThreshold,
		}
		if app.Spec.Replicas.Min != nil {
			ret.AutoScaling.Min = *app.Spec.Replicas.Min
		}
		if app.Spec.Replicas.Max != nil {
			ret.AutoScaling.Max = *app.Spec.Replicas.Max
		}
	}

	ap := model.AccessPolicy{}
	if err := convert(app.Spec.AccessPolicy, &ap); err != nil {
		return nil, fmt.Errorf("converting accessPolicy: %w", err)
	}
	ret.AccessPolicy = ap

	r := model.Resources{}
	if err := convert(app.Spec.Resources, &r); err != nil {
		return nil, fmt.Errorf("converting resources: %w", err)
	}
	ret.Resources = r

	authz, err := appAuthz(app)
	if err != nil {
		return nil, fmt.Errorf("getting authz: %w", err)
	}

	ret.Authz = authz

	for _, v := range app.Spec.Env {
		m := &model.Variable{
			Name:  v.Name,
			Value: v.Value,
		}
		ret.Variables = append(ret.Variables, m)
	}

	secrets := make([]string, 0)
	for _, filesFrom := range app.Spec.FilesFrom {
		secrets = append(secrets, filesFrom.Secret)
	}
	for _, secretName := range app.Spec.EnvFrom {
		secrets = append(secrets, secretName.Secret)
	}

	slices.Sort(secrets)
	ret.GQLVars.SecretNames = slices.Compact(secrets)

	return ret, nil
}

func toTopic(u *unstructured.Unstructured, name, team string) (*model.Topic, error) {
	topic := &kafka_nais_io_v1.Topic{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, topic); err != nil {
		return nil, fmt.Errorf("converting to application: %w", err)
	}

	ret := &model.Topic{}

	if topic.Status != nil && topic.Status.FullyQualifiedName != "" {
		ret.Name = topic.Status.FullyQualifiedName
	} else {
		ret.Name = topic.GetName()
	}

	for _, v := range topic.Spec.ACL {
		acl := &model.ACL{}
		if err := convert(v, acl); err != nil {
			return nil, fmt.Errorf("converting acl: %w", err)
		}
		if acl.Team.String() == team && acl.Application == name {
			ret.ACL = append(ret.ACL, acl)
		}
	}

	return ret, nil
}

func setStatus(app *model.App, conditions []metav1.Condition, instances []*model.Instance) {
	currentCondition := synchronizationStateCondition(conditions)
	numFailing := failing(instances)
	appState := model.AppState{
		State:  model.StateNais,
		Errors: []model.StateError{},
	}

	if currentCondition != nil {
		switch currentCondition.Reason {

		// A FailedGenerate error is almost always the user's fault.
		case sync_states.FailedGenerate:
			appState.Errors = append(appState.Errors, &model.InvalidNaisYamlError{
				Revision: app.DeployInfo.CommitSha,
				Level:    model.ErrorLevelError,
				Detail:   currentCondition.Message,
			})
			appState.State = model.StateNotnais

		// All these states can be considered transient, and indicate that there might be some internal errors going on.
		case sync_states.FailedPrepare:
			fallthrough
		case sync_states.Retrying:
			fallthrough
		case sync_states.FailedSynchronization:
			appState.Errors = append(appState.Errors, &model.SynchronizationFailingError{
				Revision: app.DeployInfo.CommitSha,
				Level:    model.ErrorLevelError,
				Detail:   currentCondition.Message,
			})
			appState.State = model.StateNotnais

		case sync_states.Synchronized:
			appState.Errors = append(appState.Errors, &model.NewInstancesFailingError{
				Revision: app.DeployInfo.CommitSha,
				Level:    model.ErrorLevelWarning,
				FailingInstances: func() []string {
					ret := make([]string, 0)
					for _, instance := range instances {
						if instance.State == model.InstanceStateFailing {
							ret = append(ret, instance.Name)
						}
					}
					return ret
				}(),
			})
			appState.State = model.StateNotnais
		}
	}

	if (len(instances) == 0 || numFailing == len(instances)) && app.AutoScaling.Min > 0 && app.AutoScaling.Max > 0 {
		appState.Errors = append(appState.Errors, &model.NoRunningInstancesError{
			Revision: app.DeployInfo.CommitSha,
			Level:    model.ErrorLevelError,
		})
		appState.State = model.StateFailing
	}

	if !strings.Contains(app.Image, "europe-north1-docker.pkg.dev") {
		parts := strings.Split(app.Image, ":")
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
			// fixme: wtf does this mean?
			repository = "confusus"
		}
		appState.Errors = append(appState.Errors, &model.DeprecatedRegistryError{
			Revision:   app.DeployInfo.CommitSha,
			Level:      model.ErrorLevelTodo,
			Registry:   registry,
			Name:       name,
			Tag:        tag,
			Repository: repository,
		})
		/*if appState.State != model.StateFailing {
			appState.State = model.StateNotnais
		}*/
	}

	deprecatedIngresses := getDeprecatedIngresses(app.Env.Name)
	for _, ingress := range app.Ingresses {
		i := strings.Join(strings.Split(ingress, ".")[1:], ".")
		for _, deprecatedIngress := range deprecatedIngresses {
			if i == deprecatedIngress {
				appState.Errors = append(appState.Errors, &model.DeprecatedIngressError{
					Revision: app.DeployInfo.CommitSha,
					Level:    model.ErrorLevelTodo,
					Ingress:  ingress,
				})
				/*if appState.State != model.StateFailing {
					appState.State = model.StateNotnais
				}*/
			}
		}
	}

	// Fjerne denna?
	if currentCondition != nil && currentCondition.Reason == sync_states.RolloutComplete && numFailing == 0 {
		if appState.State != model.StateFailing && appState.State != model.StateNotnais {
			appState.State = model.StateNais
		}
	}

	for _, rule := range app.AccessPolicy.Inbound.Rules {
		if !rule.Mutual {
			appState.Errors = append(appState.Errors, &model.InboundAccessError{
				Revision: app.DeployInfo.CommitSha,
				Level:    model.ErrorLevelWarning,
				Rule:     *rule,
			})
			if appState.State != model.StateFailing {
				appState.State = model.StateNotnais
			}
		}
	}

	for _, rule := range app.AccessPolicy.Outbound.Rules {
		if !rule.Mutual {
			appState.Errors = append(appState.Errors, &model.OutboundAccessError{
				Revision: app.DeployInfo.CommitSha,
				Level:    model.ErrorLevelWarning,
				Rule:     *rule,
			})
			if appState.State != model.StateFailing {
				appState.State = model.StateNotnais
			}
		}
	}

	app.AppState = appState
}

func failing(instances []*model.Instance) int {
	ret := 0
	for _, instance := range instances {
		if instance.State == model.InstanceStateFailing {
			ret++
		}
	}
	return ret
}

func synchronizationStateCondition(conditions []metav1.Condition) *metav1.Condition {
	for _, condition := range conditions {
		if condition.Type == "SynchronizationState" {
			return &condition
		}
	}
	return nil
}

func (c *Client) appPersistence(u *unstructured.Unstructured, topics []*model.Topic, env string) ([]model.Persistence, error) {
	app := &naisv1alpha1.Application{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, app); err != nil {
		return nil, fmt.Errorf("converting to application: %w", err)
	}

	ret := make([]model.Persistence, 0)

	if inf := c.informers[env].BucketInformer; inf != nil {
		buckets, err := inf.Lister().ByNamespace(app.GetNamespace()).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("listing buckets: %w", err)
		}
		for _, bucket := range buckets {
			b, err := model.ToBucket(bucket.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, fmt.Errorf("converting bucket: %w", err)
			}
			ret = append(ret, b)
		}
	}

	if app.Spec.GCP != nil {
		// TODO: Use SqlInstance informer for this instead?
		for _, v := range app.Spec.GCP.SqlInstances {
			sqlInstance := model.SQLInstance{}
			if err := convert(v, &sqlInstance); err != nil {
				return nil, fmt.Errorf("converting sqlInstance: %w", err)
			}
			if sqlInstance.Name == "" {
				sqlInstance.Name = app.Name
			}
			sqlInstance.ID = scalar.SqlInstanceIdent("sqlInstance_" + env + "_" + app.GetNamespace() + "_" + sqlInstance.GetName())
			ret = append(ret, sqlInstance)
		}

		for _, v := range app.Spec.GCP.BigQueryDatasets {
			bqDataset := model.BigQueryDataset{}
			if err := convert(v, &bqDataset); err != nil {
				return nil, fmt.Errorf("converting bigQueryDataset: %w", err)
			}
			ret = append(ret, bqDataset)
		}
	}

	if app.Spec.OpenSearch != nil {
		os := model.OpenSearch{
			Name:   app.Spec.OpenSearch.Instance,
			Access: app.Spec.OpenSearch.Access,
		}
		ret = append(ret, os)
	}

	if app.Spec.Kafka != nil {
		kafka := model.Kafka{
			Name:    app.Spec.Kafka.Pool,
			Streams: app.Spec.Kafka.Streams,
			Topics:  topics,
		}

		ret = append(ret, kafka)
	}

	if len(app.Spec.Redis) > 0 {
		for _, v := range app.Spec.Redis {
			redis := model.Redis{
				Name:   v.Instance,
				Access: v.Access,
			}
			ret = append(ret, redis)
		}
	}

	if app.Spec.Influx != nil {
		influx := model.InfluxDb{
			Name: app.Spec.Influx.Instance,
		}
		ret = append(ret, influx)
	}
	return ret, nil
}

func appAuthz(app *naisv1alpha1.Application) ([]model.Authz, error) {
	ret := make([]model.Authz, 0)
	if app.Spec.Azure != nil {
		isApp := app.Spec.Azure.Application != nil && app.Spec.Azure.Application.Enabled
		isSidecar := app.Spec.Azure.Sidecar != nil && app.Spec.Azure.Sidecar.Enabled
		if isApp || isSidecar {
			azureAd := model.AzureAd{}
			if err := convert(app.Spec.Azure, &azureAd); err != nil {
				return nil, fmt.Errorf("converting azureAd: %w", err)
			}
			ret = append(ret, azureAd)
		}
	}

	if app.Spec.IDPorten != nil && app.Spec.IDPorten.Enabled {
		idPorten := model.IDPorten{}
		if err := convert(app.Spec.IDPorten, &idPorten); err != nil {
			return nil, fmt.Errorf("converting idPorten: %w", err)
		}
		ret = append(ret, idPorten)
	}

	if app.Spec.Maskinporten != nil && app.Spec.Maskinporten.Enabled {
		maskinporten := model.Maskinporten{}
		if err := convert(app.Spec.Maskinporten, &maskinporten); err != nil {
			return nil, fmt.Errorf("converting maskinporten: %w", err)
		}
		ret = append(ret, maskinporten)
	}

	if app.Spec.TokenX != nil && app.Spec.TokenX.Enabled {
		tokenX := model.TokenX{}
		if err := convert(app.Spec.TokenX, &tokenX); err != nil {
			return nil, fmt.Errorf("converting tokenX: %w", err)
		}
		ret = append(ret, tokenX)
	}

	return ret, nil
}

func notFoundError(err error) bool {
	var statusError *k8serrors.StatusError
	return errors.As(err, &statusError) && statusError.ErrStatus.Reason == metav1.StatusReasonNotFound
}

func (c *Client) DeleteApp(ctx context.Context, name, team, env string) error {
	impersonatedClients, err := c.impersonationClientCreator(ctx)
	if err != nil {
		return c.error(ctx, err, "impersonation")
	}

	cli, ok := impersonatedClients[env]
	if !ok {
		return c.error(ctx, fmt.Errorf("no client set for env %q", env), "getting client")
	}

	app := cli.dynamicClient.Resource(naisv1alpha1.GroupVersion.WithResource("applications")).Namespace(team)
	if err := app.Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return c.error(ctx, err, "deleting application")
	}

	return nil
}

func (c *Client) RestartApp(ctx context.Context, name, team, env string) error {
	impersonatedClients, err := c.impersonationClientCreator(ctx)
	if err != nil {
		return c.error(ctx, err, "impersonation")
	}

	cli, ok := impersonatedClients[env]
	if !ok {
		return c.error(ctx, fmt.Errorf("no client set for env %q", env), "getting client")
	}

	b := []byte(fmt.Sprintf(`{"spec": {"template": {"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": "%s"}}}}}`, time.Now().Format(time.RFC3339)))
	app := cli.client.AppsV1().Deployments(team)
	if _, err := app.Patch(ctx, name, types.StrategicMergePatchType, b, metav1.PatchOptions{}); err != nil {
		return c.error(ctx, err, "restarting application")
	}

	return nil
}
