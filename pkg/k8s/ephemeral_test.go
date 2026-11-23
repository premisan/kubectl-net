package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestFindReusableEphemeralContainer(t *testing.T) {
	privTrue := true
	privFalse := false

	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			EphemeralContainers: []corev1.EphemeralContainer{
				{
					EphemeralContainerCommon: corev1.EphemeralContainerCommon{
						Name:  "kubectl-net-debug-aaa111",
						Image: "nicolaka/netshoot:latest",
						SecurityContext: &corev1.SecurityContext{
							Privileged: &privTrue,
						},
					},
				},
				{
					EphemeralContainerCommon: corev1.EphemeralContainerCommon{
						Name:  "kubectl-net-mysql-bbb222",
						Image: "mysql:latest",
					},
				},
				{
					EphemeralContainerCommon: corev1.EphemeralContainerCommon{
						Name:  "kubectl-net-redis-ccc333",
						Image: "redis:latest",
						SecurityContext: &corev1.SecurityContext{
							Privileged: &privFalse,
						},
					},
				},
				{
					EphemeralContainerCommon: corev1.EphemeralContainerCommon{
						Name:  "kubectl-net-psql-ddd444",
						Image: "postgres:latest",
					},
				},
				{
					EphemeralContainerCommon: corev1.EphemeralContainerCommon{
						Name:  "unrelated-debug-eee555",
						Image: "mysql:latest",
					},
				},
			},
		},
		Status: corev1.PodStatus{
			EphemeralContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "kubectl-net-debug-aaa111",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
				{
					Name:  "kubectl-net-mysql-bbb222",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
				{
					Name:  "kubectl-net-redis-ccc333",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
				{
					Name:  "kubectl-net-psql-ddd444",
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{}},
				},
				{
					Name:  "unrelated-debug-eee555",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
			},
		},
	}

	tests := []struct {
		name              string
		image             string
		requirePrivileged bool
		want              string
	}{
		{
			name:              "reuse matching privileged netshoot container",
			image:             "nicolaka/netshoot:latest",
			requirePrivileged: true,
			want:              "kubectl-net-debug-aaa111",
		},
		{
			name:              "reuse matching non-privileged mysql container",
			image:             "mysql:latest",
			requirePrivileged: false,
			want:              "kubectl-net-mysql-bbb222",
		},
		{
			name:              "do not reuse netshoot container when requesting different image",
			image:             "mariadb:latest",
			requirePrivileged: false,
			want:              "",
		},
		{
			name:              "do not reuse terminated psql container",
			image:             "postgres:latest",
			requirePrivileged: false,
			want:              "",
		},
		{
			name:              "do not reuse non-privileged container when privileged is required",
			image:             "redis:latest",
			requirePrivileged: true,
			want:              "",
		},
		{
			name:              "reuse privileged container when non-privileged is requested with same image",
			image:             "nicolaka/netshoot:latest",
			requirePrivileged: false,
			want:              "kubectl-net-debug-aaa111",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findReusableEphemeralContainer(pod, tt.image, tt.requirePrivileged)
			if got != tt.want {
				t.Fatalf("findReusableEphemeralContainer(%q, %v) = %q, want %q", tt.image, tt.requirePrivileged, got, tt.want)
			}
		})
	}
}
