package application

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestIndexIngressHostsSkipsHostlessRules(t *testing.T) {
	for _, tt := range []struct {
		name  string
		rules []any
		want  []string
	}{
		{name: "no rules", want: []string{}},
		{name: "absent host", rules: []any{map[string]any{}}, want: []string{}},
		{name: "null host", rules: []any{map[string]any{"host": nil}}, want: []string{}},
		{name: "empty host", rules: []any{map[string]any{"host": ""}}, want: []string{}},
		{
			name: "hostless and named rules",
			rules: []any{
				map[string]any{},
				map[string]any{"host": nil},
				map[string]any{"host": "APP.EXAMPLE.COM."},
			},
			want: []string{"app.example.com"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ingress := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "networking.k8s.io/v1",
				"kind":       "Ingress",
				"spec":       map[string]any{},
			}}
			if tt.rules != nil {
				if err := unstructured.SetNestedSlice(ingress.Object, tt.rules, "spec", "rules"); err != nil {
					t.Fatal(err)
				}
			}
			transformed, err := transformIngress(ingress)
			if err != nil {
				t.Fatal(err)
			}
			hosts, err := indexIngressHosts(transformed)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, hosts); diff != "" {
				t.Errorf("unexpected indexed hosts (-want +got):\n%s", diff)
			}
		})
	}
}
