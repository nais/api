package kubernetes

import (
	"fmt"
	"strings"

	sql_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/sql/v1beta1"
	storage_cnrm_cloud_gogle_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/storage/v1beta1"
	liberator_aiven_io_v1alpha1 "github.com/nais/liberator/pkg/apis/aiven.io/v1alpha1"
	bigquery_nais_io_v1 "github.com/nais/liberator/pkg/apis/google.nais.io/v1"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func NewScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()

	funcs := []func(s *runtime.Scheme) error{
		nais_io_v1.AddToScheme,
		nais_io_v1alpha1.AddToScheme,
		kafka_nais_io_v1.AddToScheme,
		sql_cnrm_cloud_google_com_v1beta1.AddToScheme,
		storage_cnrm_cloud_gogle_com_v1beta1.AddToScheme,
		bigquery_nais_io_v1.AddToScheme,
		liberator_aiven_io_v1alpha1.AddToScheme,
		unleash_nais_io_v1.AddToScheme,
		corev1.AddToScheme,
		appsv1.AddToScheme,
		netv1.AddToScheme,
		batchv1.AddToScheme,
	}

	for _, f := range funcs {
		// These unclude known types which converts into concrete types
		// We would like them to only operate on unstructured types
		if err := withoutKnownTypes(f)(scheme); err != nil {
			return nil, err
		}
	}

	return scheme, nil
}

func withoutKnownTypes(ats func(*runtime.Scheme) error) func(*runtime.Scheme) error {
	return func(s *runtime.Scheme) error {
		scheme := runtime.NewScheme()
		if err := ats(scheme); err != nil {
			return err
		}

		for gvk := range scheme.AllKnownTypes() {
			if s.Recognizes(gvk) {
				continue
			}
			// Ensure we are always supporting unstructured objects
			// This to prevent various problems with the fake client
			if strings.HasSuffix(gvk.Kind, "List") {
				s.AddKnownTypeWithName(gvk, &unstructured.UnstructuredList{})
				continue
			}
			s.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		}

		for gvk := range s.AllKnownTypes() {
			fmt.Println(gvk)
		}

		return nil
	}
}
