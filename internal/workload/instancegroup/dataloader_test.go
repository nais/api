package instancegroup

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestTransformReplicaSetIdempotent verifies that transformReplicaSet is idempotent.
// The Kubernetes informer's WatchList code path can run the transformer twice
// on the same object: once in a temporary store, and again in DeltaFIFO.Replace().
// If the transformer is not idempotent, the second run could lose data.
func TestTransformReplicaSetIdempotent(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "ReplicaSet",
			"metadata": map[string]any{
				"name":      "my-app-abc123",
				"namespace": "my-team",
				"labels": map[string]any{
					"app":               "my-app",
					"team":              "my-team",
					"some-other-label":  "should-be-removed",
					"pod-template-hash": "abc123",
				},
				"annotations": map[string]any{
					"deployment.kubernetes.io/revision":         "3",
					"deployment.kubernetes.io/desired-replicas": "2",
					"deployment.kubernetes.io/max-replicas":     "3",
				},
				"managedFields": []any{
					map[string]any{"manager": "kube-controller-manager"},
				},
				"generateName": "my-app-",
				"ownerReferences": []any{
					map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"name":       "my-app",
						"uid":        "some-uid",
					},
				},
			},
			"spec": map[string]any{
				"replicas": int64(2),
				"selector": map[string]any{
					"matchLabels": map[string]any{
						"app":               "my-app",
						"pod-template-hash": "abc123",
					},
				},
				"template": map[string]any{
					"metadata": map[string]any{
						"labels": map[string]any{
							"app":               "my-app",
							"team":              "my-team",
							"pod-template-hash": "abc123",
						},
					},
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name":  "my-app",
								"image": "my-registry/my-app:v1.2.3",
								"env": []any{
									map[string]any{
										"name":  "DATABASE_URL",
										"value": "postgres://localhost/mydb",
									},
								},
								"envFrom": []any{
									map[string]any{
										"secretRef": map[string]any{"name": "my-secret"},
									},
								},
								"volumeMounts": []any{
									map[string]any{
										"name":      "config-vol",
										"mountPath": "/etc/config",
									},
								},
								"resources": map[string]any{
									"requests": map[string]any{
										"cpu":    "100m",
										"memory": "128Mi",
									},
								},
								"ports": []any{
									map[string]any{
										"containerPort": int64(8080),
									},
								},
							},
						},
						"volumes": []any{
							map[string]any{
								"name": "config-vol",
								"configMap": map[string]any{
									"name": "my-config",
								},
							},
						},
						"serviceAccountName": "my-app",
						"dnsPolicy":          "ClusterFirst",
					},
				},
			},
			"status": map[string]any{
				"replicas":           int64(2),
				"readyReplicas":      int64(2),
				"availableReplicas":  int64(2),
				"observedGeneration": int64(5),
				"conditions": []any{
					map[string]any{
						"type":   "Available",
						"status": "True",
					},
				},
			},
		},
	}

	// First transform
	result1, err := transformReplicaSet(obj)
	if err != nil {
		t.Fatalf("first transformReplicaSet call failed: %v", err)
	}

	rs1 := result1.(*unstructured.Unstructured)

	// Verify first transform stripped unnecessary fields
	verifyTransformedRS(t, rs1, "first")

	// Second transform (idempotency check)
	result2, err := transformReplicaSet(rs1)
	if err != nil {
		t.Fatalf("second transformReplicaSet call failed: %v", err)
	}

	rs2 := result2.(*unstructured.Unstructured)

	// Verify second transform produces identical result
	verifyTransformedRS(t, rs2, "second")

	// Compare the two results — they must be identical
	if diff := cmp.Diff(rs1.Object, rs2.Object); diff != "" {
		t.Fatalf("transformReplicaSet is NOT idempotent, diff (-first +second):\n%s", diff)
	}
}

func TestTransformReplicaSetMinimal(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "ReplicaSet",
			"metadata": map[string]any{
				"name":      "minimal-rs",
				"namespace": "team",
			},
			"spec": map[string]any{
				"replicas": int64(1),
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name":  "app",
								"image": "nginx:latest",
							},
						},
					},
				},
			},
			"status": map[string]any{
				"replicas":      int64(1),
				"readyReplicas": int64(1),
			},
		},
	}

	result, err := transformReplicaSet(obj)
	if err != nil {
		t.Fatalf("transformReplicaSet failed: %v", err)
	}

	rs := result.(*unstructured.Unstructured)

	// Should have containers with just name and image
	containers, _, _ := unstructured.NestedSlice(rs.Object, "spec", "template", "spec", "containers")
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	c := containers[0].(map[string]any)
	if c["name"] != "app" {
		t.Errorf("container name = %q, want %q", c["name"], "app")
	}
	if c["image"] != "nginx:latest" {
		t.Errorf("container image = %q, want %q", c["image"], "nginx:latest")
	}

	// Should not have resources, ports, etc.
	if _, ok := c["resources"]; ok {
		t.Error("resources should have been stripped")
	}
	if _, ok := c["ports"]; ok {
		t.Error("ports should have been stripped")
	}
}

func verifyTransformedRS(t *testing.T, rs *unstructured.Unstructured, label string) {
	t.Helper()

	// Labels: only "app" and "team" should remain
	labels := rs.GetLabels()
	if labels["app"] != "my-app" {
		t.Errorf("%s: label 'app' = %q, want %q", label, labels["app"], "my-app")
	}
	if labels["team"] != "my-team" {
		t.Errorf("%s: label 'team' = %q, want %q", label, labels["team"], "my-team")
	}
	if _, ok := labels["some-other-label"]; ok {
		t.Errorf("%s: label 'some-other-label' should have been removed", label)
	}
	if _, ok := labels["pod-template-hash"]; ok {
		t.Errorf("%s: label 'pod-template-hash' should have been removed", label)
	}

	// Annotations: only revision should remain
	annotations := rs.GetAnnotations()
	if annotations["deployment.kubernetes.io/revision"] != "3" {
		t.Errorf("%s: annotation 'revision' = %q, want %q", label, annotations["deployment.kubernetes.io/revision"], "3")
	}
	if _, ok := annotations["deployment.kubernetes.io/desired-replicas"]; ok {
		t.Errorf("%s: annotation 'desired-replicas' should have been removed", label)
	}

	// managedFields and generateName should be removed
	if _, exists, _ := unstructured.NestedSlice(rs.Object, "metadata", "managedFields"); exists {
		t.Errorf("%s: managedFields should have been removed", label)
	}
	if _, exists, _ := unstructured.NestedString(rs.Object, "metadata", "generateName"); exists {
		t.Errorf("%s: generateName should have been removed", label)
	}

	// spec.replicas should be preserved
	replicas, _, _ := unstructured.NestedInt64(rs.Object, "spec", "replicas")
	if replicas != 2 {
		t.Errorf("%s: spec.replicas = %d, want 2", label, replicas)
	}

	// spec.selector should be removed (not needed)
	if _, exists, _ := unstructured.NestedMap(rs.Object, "spec", "selector"); exists {
		t.Errorf("%s: spec.selector should have been removed", label)
	}

	// Containers should only keep name, image, env, envFrom, volumeMounts
	containers, _, _ := unstructured.NestedSlice(rs.Object, "spec", "template", "spec", "containers")
	if len(containers) != 1 {
		t.Fatalf("%s: expected 1 container, got %d", label, len(containers))
	}
	c := containers[0].(map[string]any)
	if c["name"] != "my-app" {
		t.Errorf("%s: container name = %q", label, c["name"])
	}
	if c["image"] != "my-registry/my-app:v1.2.3" {
		t.Errorf("%s: container image = %q", label, c["image"])
	}
	if _, ok := c["env"]; !ok {
		t.Errorf("%s: env should be preserved", label)
	}
	if _, ok := c["envFrom"]; !ok {
		t.Errorf("%s: envFrom should be preserved", label)
	}
	if _, ok := c["volumeMounts"]; !ok {
		t.Errorf("%s: volumeMounts should be preserved", label)
	}
	if _, ok := c["resources"]; ok {
		t.Errorf("%s: resources should have been stripped", label)
	}
	if _, ok := c["ports"]; ok {
		t.Errorf("%s: ports should have been stripped", label)
	}

	// Volumes should be preserved
	volumes, ok, _ := unstructured.NestedSlice(rs.Object, "spec", "template", "spec", "volumes")
	if !ok || len(volumes) != 1 {
		t.Errorf("%s: expected 1 volume, got %d (ok=%v)", label, len(volumes), ok)
	}

	// Template labels should be preserved
	templateLabels, _, _ := unstructured.NestedStringMap(rs.Object, "spec", "template", "metadata", "labels")
	if templateLabels["app"] != "my-app" {
		t.Errorf("%s: template label 'app' = %q", label, templateLabels["app"])
	}

	// ownerReferences should be preserved
	ownerRefs, ok, _ := unstructured.NestedSlice(rs.Object, "metadata", "ownerReferences")
	if !ok || len(ownerRefs) != 1 {
		t.Errorf("%s: expected 1 ownerReference, got %d (ok=%v)", label, len(ownerRefs), ok)
	}

	// Status: only replicas and readyReplicas
	statusReplicas, _, _ := unstructured.NestedInt64(rs.Object, "status", "replicas")
	if statusReplicas != 2 {
		t.Errorf("%s: status.replicas = %d, want 2", label, statusReplicas)
	}
	readyReplicas, _, _ := unstructured.NestedInt64(rs.Object, "status", "readyReplicas")
	if readyReplicas != 2 {
		t.Errorf("%s: status.readyReplicas = %d, want 2", label, readyReplicas)
	}
	if _, exists, _ := unstructured.NestedInt64(rs.Object, "status", "availableReplicas"); exists {
		t.Errorf("%s: status.availableReplicas should have been removed", label)
	}
	if _, exists, _ := unstructured.NestedSlice(rs.Object, "status", "conditions"); exists {
		t.Errorf("%s: status.conditions should have been removed", label)
	}

	// serviceAccountName and dnsPolicy should be removed
	if _, exists, _ := unstructured.NestedString(rs.Object, "spec", "template", "spec", "serviceAccountName"); exists {
		t.Errorf("%s: serviceAccountName should have been removed", label)
	}
	if _, exists, _ := unstructured.NestedString(rs.Object, "spec", "template", "spec", "dnsPolicy"); exists {
		t.Errorf("%s: dnsPolicy should have been removed", label)
	}
}
