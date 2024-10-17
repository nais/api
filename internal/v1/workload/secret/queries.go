package secret

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/auditv1"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
)

var ErrSecretUnmanaged = errors.New("secret is not managed by console")

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination) (*SecretConnection, error) {
	all := fromContext(ctx).secretWatcher.GetByNamespace(teamSlug.String())
	secrets := pagination.Slice(watcher.Objects(all), page)
	return pagination.NewConnection(secrets, page, int32(len(all))), nil
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Secret, error) {
	return fromContext(ctx).secretWatcher.Get(environment, teamSlug.String(), name)
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Secret, error) {
	teamSlug, env, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, teamSlug, env, name)
}

func GetSecretValues(ctx context.Context, teamSlug slug.Slug, environmentName, name string) ([]*SecretValue, error) {
	client, err := fromContext(ctx).secretWatcher.ImpersonatedClient(ctx, environmentName)
	if err != nil {
		return nil, err
	}

	u, err := client.Namespace(teamSlug.String()).Get(ctx, name, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	data, _, err := unstructured.NestedStringMap(u.Object, "data")
	if err != nil {
		return nil, err
	}

	vars := make([]*SecretValue, 0, len(data))
	for k, v := range data {
		val, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, err
		}

		vars = append(vars, &SecretValue{
			Name:  k,
			Value: string(val),
		})
	}

	slices.SortFunc(vars, func(a, b *SecretValue) int {
		return strings.Compare(a.Name, b.Name)
	})

	return vars, nil
}

func Create(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Secret, error) {
	client, err := fromContext(ctx).secretWatcher.ImpersonatedClient(ctx, environment)
	if err != nil {
		return nil, err
	}

	if nameErrs := validation.IsDNS1123Subdomain(name); len(nameErrs) > 0 {
		return nil, fmt.Errorf("invalid name %q: %s", name, strings.Join(nameErrs, ", "))
	}

	actor := authz.ActorFromContext(ctx)

	secret := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:        name,
			Namespace:   teamSlug.String(),
			Annotations: annotations(actor.User.Identity()),
			Labels:      labels(),
		},
		Type: corev1.SecretTypeOpaque,
	}

	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	if err != nil {
		return nil, err
	}

	s, err := client.Namespace(teamSlug.String()).Create(ctx, &unstructured.Unstructured{Object: u}, v1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("%q: %w", name, ErrSecretUnmanaged)
		}
		return nil, fmt.Errorf("creating secret: %w", err)
	}

	err = auditv1.Create(ctx, auditv1.CreateInput{
		Action:       auditActionCreateSecret,
		Actor:        actor.User,
		ResourceType: auditResourceTypeSecret,
		ResourceName: name,
		TeamSlug:     ptr.To(teamSlug),
	})
	if err != nil {
		fromContext(ctx).log.WithError(err).Errorf("unable to create audit log entry")
	}

	retVal, ok := toGraphSecret(s, environment)
	if !ok {
		return nil, fmt.Errorf("failed to convert secret to graph secret")
	}
	return retVal, nil
}

func SetSecretValue(ctx context.Context, teamSlug slug.Slug, environment, name string, value *SecretValueInput) (*Secret, error) {
	client, err := fromContext(ctx).secretWatcher.ImpersonatedClient(ctx, environment)
	if err != nil {
		return nil, err
	}

	if err := validateSecretValue(value); err != nil {
		return nil, err
	}

	obj, err := client.Namespace(teamSlug.String()).Get(ctx, name, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	existingSecret := &corev1.Secret{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &existingSecret); err != nil {
		return nil, err
	}

	if !secretIsManagedByConsole(obj) {
		return nil, fmt.Errorf("%q: %w", name, ErrSecretUnmanaged)
	}

	secretValues, err := GetSecretValues(ctx, teamSlug, environment, name)
	if err != nil {
		return nil, err
	}

	existing := false
	for i, v := range secretValues {
		if v.Name == value.Name {
			existing = true
			secretValues[i].Value = value.Value
			break
		}
	}

	if !existing {
		secretValues = append(secretValues, &SecretValue{
			Name:  value.Name,
			Value: value.Value,
		})
	}

	actor := authz.ActorFromContext(ctx)

	existingSecret.Annotations = annotations(actor.User.Identity())
	existingSecret.Labels = labels()
	existingSecret.Data = secretTupleToMap(secretValues)

	unstructeredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(existingSecret)
	if err != nil {
		return nil, err
	}

	u := &unstructured.Unstructured{Object: unstructeredMap}
	if _, err := client.Namespace(teamSlug.String()).Update(ctx, u, v1.UpdateOptions{}); err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func RemoveSecretValue(ctx context.Context, teamSlug slug.Slug, environment, secretName, valueName string) (*Secret, error) {
	client, err := fromContext(ctx).secretWatcher.ImpersonatedClient(ctx, environment)
	if err != nil {
		return nil, err
	}

	obj, err := client.Namespace(teamSlug.String()).Get(ctx, secretName, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	existingSecret := &corev1.Secret{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &existingSecret); err != nil {
		return nil, err
	}

	if !secretIsManagedByConsole(obj) {
		return nil, fmt.Errorf("%q: %w", secretName, ErrSecretUnmanaged)
	}

	secretValues, err := GetSecretValues(ctx, teamSlug, environment, secretName)
	if err != nil {
		return nil, err
	}

	secretMap := secretTupleToMap(secretValues)
	if _, exists := secretMap[valueName]; !exists {
		return nil, apierror.Errorf("No such secret value exists: %q", valueName)
	}

	delete(secretMap, valueName)

	actor := authz.ActorFromContext(ctx)

	existingSecret.Annotations = annotations(actor.User.Identity())
	existingSecret.Labels = labels()
	existingSecret.Data = secretMap

	unstructeredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(existingSecret)
	if err != nil {
		return nil, err
	}

	u := &unstructured.Unstructured{Object: unstructeredMap}
	if _, err := client.Namespace(teamSlug.String()).Update(ctx, u, v1.UpdateOptions{}); err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, secretName)
}

func Delete(ctx context.Context, teamSlug slug.Slug, environment, name string) error {
	sw := fromContext(ctx).secretWatcher
	if _, err := sw.Get(environment, teamSlug.String(), name); err != nil {
		return err
	}

	if err := sw.Delete(ctx, environment, teamSlug.String(), name); err != nil {
		return err
	}

	return nil
}

func annotations(user string) map[string]string {
	return map[string]string{
		secretAnnotationLastModifiedBy: user,
		secretAnnotationLastModifiedAt: time.Now().Format(time.RFC3339),
		"reloader.stakater.com/match":  "true",
	}
}

func labels() map[string]string {
	return map[string]string{
		secretLabelManagedByKey: secretLabelManagedByVal,
	}
}

func validateSecretValue(value *SecretValueInput) error {
	if len(value.Name) > validation.DNS1123SubdomainMaxLength {
		return fmt.Errorf("%q is too long: %d characters, max %d", value.Name, len(value.Name), validation.DNS1123SubdomainMaxLength)
	}

	if isEnvVarName := validation.IsEnvVarName(value.Name); len(isEnvVarName) > 0 {
		return fmt.Errorf("%q: %s", value.Name, strings.Join(isEnvVarName, ", "))
	}

	return nil
}

func secretTupleToMap(data []*SecretValue) map[string][]byte {
	ret := make(map[string][]byte, len(data))
	for _, tuple := range data {
		ret[tuple.Name] = []byte(tuple.Value)
	}
	return ret
}

func secretIsManagedByConsole(secret *unstructured.Unstructured) bool {
	labels := secret.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	secretLabel, ok := labels[secretLabelManagedByKey]
	hasConsoleLabel := ok && secretLabel == secretLabelManagedByVal
	secretType, _, _ := unstructured.NestedString(secret.Object, "type")
	isOpaque := secretType == string(corev1.SecretTypeOpaque) || secretType == "kubernetes.io/Opaque"
	hasOwnerReferences := len(secret.GetOwnerReferences()) > 0
	hasFinalizers := len(secret.GetFinalizers()) > 0

	typeLabel, ok := labels["type"]
	isJwker := ok && typeLabel == "jwker.nais.io"

	return hasConsoleLabel && isOpaque && !hasOwnerReferences && !hasFinalizers && !isJwker
}
