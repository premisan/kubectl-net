package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFindTargetContainer(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app-main"},
				{Name: "sidecar-proxy"},
			},
		},
	}

	// 1. Default to first container when empty
	cName, err := FindTargetContainer(pod, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cName != "app-main" {
		t.Errorf("expected 'app-main', got '%s'", cName)
	}

	// 2. Exact match
	cName, err = FindTargetContainer(pod, "sidecar-proxy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cName != "sidecar-proxy" {
		t.Errorf("expected 'sidecar-proxy', got '%s'", cName)
	}

	// 3. Not found
	_, err = FindTargetContainer(pod, "nonexistent")
	if err == nil {
		t.Errorf("expected error for nonexistent container, got nil")
	}

	// 4. Pod with no containers
	emptyPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "empty"}}
	_, err = FindTargetContainer(emptyPod, "")
	if err == nil {
		t.Errorf("expected error for empty pod, got nil")
	}
}
