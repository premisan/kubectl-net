package k8s

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ClientContext encapsulates K8s clientset, rest config, and resolved namespace
type ClientContext struct {
	Clientset kubernetes.Interface
	Config    *rest.Config
	Namespace string
}

// NewClientContext loads kubeconfig, handles context/namespace overrides, and initializes clientset
func NewClientContext(kubeconfigPath, contextOverride, namespaceOverride string) (*ClientContext, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	} else if envPath := os.Getenv("KUBECONFIG"); envPath != "" {
		loadingRules.Precedence = append([]string{envPath}, loadingRules.Precedence...)
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	if contextOverride != "" {
		configOverrides.CurrentContext = contextOverride
	}
	if namespaceOverride != "" {
		configOverrides.Context.Namespace = namespaceOverride
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load raw kubeconfig: %w", err)
	}

	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build client config: %w", err)
	}

	// Resolve namespace
	namespace := namespaceOverride
	if namespace == "" {
		if rawConfig.Contexts[rawConfig.CurrentContext] != nil && rawConfig.Contexts[rawConfig.CurrentContext].Namespace != "" {
			namespace = rawConfig.Contexts[rawConfig.CurrentContext].Namespace
		} else {
			namespace = "default"
		}
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	return &ClientContext{
		Clientset: clientset,
		Config:    restConfig,
		Namespace: namespace,
	}, nil
}

// GetPod retrieves a Pod and verifies it is in Running state
func (c *ClientContext) GetPod(ctx context.Context, podName string) (*corev1.Pod, error) {
	pod, err := c.Clientset.CoreV1().Pods(c.Namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod '%s' in namespace '%s': %w", podName, c.Namespace, err)
	}
	if pod.Status.Phase != corev1.PodRunning {
		return nil, fmt.Errorf("pod '%s' is not running (current phase: %s)", podName, pod.Status.Phase)
	}
	return pod, nil
}

// FindTargetContainer returns the verified target container name or defaults to the first container
func FindTargetContainer(pod *corev1.Pod, requestedContainer string) (string, error) {
	if len(pod.Spec.Containers) == 0 {
		return "", fmt.Errorf("pod %s has no containers", pod.Name)
	}

	if requestedContainer == "" {
		return pod.Spec.Containers[0].Name, nil
	}

	for _, c := range pod.Spec.Containers {
		if c.Name == requestedContainer {
			return c.Name, nil
		}
	}

	return "", fmt.Errorf("container '%s' not found in pod '%s'", requestedContainer, pod.Name)
}
