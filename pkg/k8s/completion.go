package k8s

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CompletePods returns a list of running Pods for dynamic shell autocompletion
func CompletePods(toComplete string, namespaceOverride, kubeconfigPath, contextOverride string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	clientCtx, err := NewClientContext(kubeconfigPath, contextOverride, namespaceOverride)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	pods, err := clientCtx.Clientset.CoreV1().Pods(clientCtx.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var results []string
	for _, p := range pods.Items {
		if strings.HasPrefix(p.Name, toComplete) {
			// Format: "pod-name\tStatus: Running" (Zsh/Bash displays description)
			results = append(results, p.Name+"\t"+string(p.Status.Phase))
		}
	}

	return results, cobra.ShellCompDirectiveNoFileComp
}

// CompleteNamespaces returns a list of namespaces for dynamic shell autocompletion
func CompleteNamespaces(toComplete string, kubeconfigPath, contextOverride string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	clientCtx, err := NewClientContext(kubeconfigPath, contextOverride, "")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	namespaces, err := clientCtx.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var results []string
	for _, ns := range namespaces.Items {
		if strings.HasPrefix(ns.Name, toComplete) {
			results = append(results, ns.Name)
		}
	}

	return results, cobra.ShellCompDirectiveNoFileComp
}

// CompleteContainers returns a list of container names in a pod for dynamic autocompletion
func CompleteContainers(podName, namespaceOverride, kubeconfigPath, contextOverride string) ([]string, cobra.ShellCompDirective) {
	if podName == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	clientCtx, err := NewClientContext(kubeconfigPath, contextOverride, namespaceOverride)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	pod, err := clientCtx.Clientset.CoreV1().Pods(clientCtx.Namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var results []string
	for _, c := range pod.Spec.Containers {
		results = append(results, c.Name)
	}

	return results, cobra.ShellCompDirectiveNoFileComp
}
