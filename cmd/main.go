package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath" // Used for wildcard matching (Glob/Match)
	"strings"
	"syscall"
	"time"

	"github.com/daigoro86dev/k8s-healer/pkg/healer"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfigPath string
	namespaces     string
	healCooldown   time.Duration
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "k8s-healer",
	Short: "A Kubernetes CLI tool that watches for and heals unhealthy pods.",
	Long: `k8s-healer monitors specified Kubernetes namespaces for persistently unhealthy pods (e.g., in CrashLoopBackOff) 
and performs a healing action by deleting the pod, forcing its controller to recreate it.

The -n/--namespaces flag supports comma-separated values and simple wildcards (*).

Usage Examples:
  k8s-healer -n prod,staging              # Watch specific namespaces
  k8s-healer -n 'app-*-dev,kube-*'        # Watch namespaces matching wildcards
  k8s-healer                              # Watch all namespaces
  k8s-healer -k /path/to/my/kubeconfig    # Use specific kubeconfig
`,
	Run: func(cmd *cobra.Command, args []string) {
		startHealer()
	},
}

func init() {
	// Global flags handled by Cobra
	rootCmd.PersistentFlags().StringVarP(&kubeconfigPath, "kubeconfig", "k", "", "Path to the kubeconfig file (defaults to standard locations).")
	rootCmd.PersistentFlags().StringVarP(&namespaces, "namespaces", "n", "", "Comma-separated list of namespaces/workspaces to watch (e.g., 'prod,staging'). Supports wildcards (*). Defaults to all namespaces if empty.")
	rootCmd.PersistentFlags().DurationVar(&healCooldown, "heal-cooldown", 10*time.Minute,
		"Minimum time between healing the same Pod (e.g. 10m, 30s).")
}

// resolveWildcardNamespaces connects to the cluster, lists all namespaces, and returns a concrete list
// based on the input patterns, handling wildcards using filepath.Match.
func resolveWildcardNamespaces(kubeconfigPath, namespacesInput string) ([]string, error) {
	if namespacesInput == "" {
		return []string{}, nil // Return empty list, signaling the healer to watch all.
	}

	patterns := strings.Split(namespacesInput, ",")
	for i, p := range patterns {
		patterns[i] = strings.TrimSpace(p)
	}

	// Check if any pattern contains a wildcard. If not, just return the list of patterns.
	needsResolution := false
	for _, p := range patterns {
		if strings.Contains(p, "*") {
			needsResolution = true
			break
		}
	}
	if !needsResolution {
		return patterns, nil
	}

	// --- Connect to Kubernetes to list existing namespaces ---

	var config *rest.Config
	var err error

	if kubeconfigPath != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", "")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to build Kubernetes config for resolution: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset for resolution: %w", err)
	}

	// List all namespaces in the cluster
	nsList, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces for wildcard resolution: %w", err)
	}

	resolvedNamespaces := make(map[string]bool)

	// Match existing namespaces against all patterns
	for _, ns := range nsList.Items {
		for _, pattern := range patterns {
			match, err := filepath.Match(pattern, ns.Name)
			if err != nil {
				// This shouldn't happen with simple glob patterns
				fmt.Printf("Warning: Invalid wildcard pattern '%s': %v\n", pattern, err)
				continue
			}
			if match {
				resolvedNamespaces[ns.Name] = true
			}
		}
	}

	// Convert map keys to slice
	var finalNsList []string
	for ns := range resolvedNamespaces {
		finalNsList = append(finalNsList, ns)
	}

	if len(finalNsList) > 0 {
		fmt.Printf("Wildcards resolved. Watching %d namespaces: [%s]\n", len(finalNsList), strings.Join(finalNsList, ", "))
	} else {
		fmt.Println("Warning: Wildcard patterns did not match any existing namespaces.")
	}

	return finalNsList, nil
}

// startHealer parses the flags, initializes the healer, and manages the shutdown signals.
func startHealer() {
	// Resolve the raw namespace input (including wildcards) into a concrete list of existing namespaces
	nsList, err := resolveWildcardNamespaces(kubeconfigPath, namespaces)
	if err != nil {
		fmt.Printf("Error resolving namespaces: %v\n", err)
		os.Exit(1)
	}

	// Initialize the Healer module. This connects to Kubernetes.
	healer, err := healer.NewHealer(kubeconfigPath, nsList)
	if err != nil {
		fmt.Printf("Error setting up Kubernetes client: %v\n", err)
		os.Exit(1)
	}

	healer.HealCooldown = healCooldown
	// Setup signal handling (SIGINT/Ctrl+C and SIGTERM) for graceful shutdown.
	termCh := make(chan os.Signal, 1)
	signal.Notify(termCh, syscall.SIGINT, syscall.SIGTERM)

	// Start the main watch loop in a goroutine. This will start the informers.
	go healer.Watch()

	// Wait for termination signal
	<-termCh
	fmt.Println("\nTermination signal received. Shutting down healer...")

	// Close the StopCh channel to signal all concurrent informers to stop gracefully.
	close(healer.StopCh)

	// Give informers a moment to stop before exiting the process.
	time.Sleep(1 * time.Second)
	fmt.Println("Healer stopped.")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
