package healer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/daigoro86dev/k8s-healer/pkg/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// Healer holds the Kubernetes client and configuration for watching.
type Healer struct {
	ClientSet    *kubernetes.Clientset
	Namespaces   []string
	StopCh       chan struct{}
	HealedPods   map[string]time.Time // Tracks recently healed pods
	HealCooldown time.Duration
}

// NewHealer initializes the Kubernetes client configuration using kubeconfig or in-cluster settings.
func NewHealer(kubeconfigPath string, namespaces []string) (*Healer, error) {
	var config *rest.Config
	var err error

	// Try to load configuration from the specified path, or default locations
	if kubeconfigPath != "" {
		// Use explicit kubeconfig path if provided
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	} else {
		// Fallback to in-cluster config or default ~/.kube/config
		config, err = clientcmd.BuildConfigFromFlags("", "")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to build Kubernetes config: %w", err)
	}

	// Create the clientset used for making API calls
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}

	return &Healer{
		ClientSet:    clientset,
		Namespaces:   namespaces,
		StopCh:       make(chan struct{}),
		HealedPods:   make(map[string]time.Time),
		HealCooldown: 10 * time.Minute, // default cooldown
	}, nil
}

// Watch starts the informer loop for all configured namespaces concurrently.
func (h *Healer) Watch() {
	// If no namespaces are provided, default to watching all namespaces
	if len(h.Namespaces) == 0 {
		fmt.Println("No namespaces specified. Watching all namespaces (using NamespaceAll).")
		h.Namespaces = []string{metav1.NamespaceAll}
	}

	fmt.Printf("Starting healer to watch namespaces: [%s]\n", strings.Join(h.Namespaces, ", "))

	h.startHealCacheCleaner()

	// Start a separate goroutine for the informer watch in each namespace
	for _, ns := range h.Namespaces {
		go h.watchSingleNamespace(ns)
	}

	// Block the main goroutine until the StopCh channel is closed (on SIGINT/SIGTERM)
	<-h.StopCh
}

// watchSingleNamespace sets up a Pod Informer for one namespace.
func (h *Healer) watchSingleNamespace(namespace string) {
	// Create a SharedInformerFactory scoped to the namespace, with a 30s resync period
	factory := informers.NewSharedInformerFactoryWithOptions(h.ClientSet, time.Second*30, informers.WithNamespace(namespace))

	// Get the Pod Informer
	podInformer := factory.Core().V1().Pods().Informer()

	// Register event handlers
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		// We use UpdateFunc because a Pod becomes unhealthy (e.g., CrashLoopBackOff) after its initial creation
		UpdateFunc: func(oldObj, newObj interface{}) {
			newPod := newObj.(*v1.Pod)
			h.checkAndHealPod(newPod)
		},
	})

	// Start the informer and wait for the cache to be synced
	factory.Start(h.StopCh)
	if !cache.WaitForCacheSync(h.StopCh, podInformer.HasSynced) {
		fmt.Printf("Error syncing cache for namespace %s. Exiting watch.\n", namespace)
		return
	}

	fmt.Printf("✅ Successfully synced cache and started watching namespace: %s\n", namespace)
}

// checkAndHealPod checks a Pod's health and executes deletion if necessary.
func (h *Healer) checkAndHealPod(pod *v1.Pod) {
	// Skip unmanaged pods
	if len(pod.OwnerReferences) == 0 {
		return
	}

	// Skip if recently healed
	podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	if lastHeal, ok := h.HealedPods[podKey]; ok {
		if time.Since(lastHeal) < h.HealCooldown {
			fmt.Printf("   [SKIP] ⏳ Pod %s was healed %.0f seconds ago — skipping re-heal.\n",
				podKey, time.Since(lastHeal).Seconds())
			return
		}
	}

	if util.IsUnhealthy(pod) {
		reason := util.GetHealReason(pod)
		fmt.Printf("\n!!! HEALING ACTION REQUIRED !!!\n")
		fmt.Printf("    Pod: %s\n", podKey)
		fmt.Printf("    Reason: %s\n", reason)

		h.triggerPodDeletion(pod)

		// Record the healing timestamp
		h.HealedPods[podKey] = time.Now()

		fmt.Printf("!!! HEALING ACTION COMPLETE !!!\n\n")
	}
}

func (h *Healer) startHealCacheCleaner() {
	ticker := time.NewTicker(30 * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				now := time.Now()
				for key, t := range h.HealedPods {
					if now.Sub(t) > 2*h.HealCooldown {
						delete(h.HealedPods, key)
					}
				}
			case <-h.StopCh:
				ticker.Stop()
				return
			}
		}
	}()
}

// triggerPodDeletion deletes the Pod, relying on the managing controller to recreate a fresh one.
func (h *Healer) triggerPodDeletion(pod *v1.Pod) {
	// Use a context with timeout for the API call to prevent indefinite hangs
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Perform the API Delete call
	err := h.ClientSet.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})

	if err != nil {
		fmt.Printf("   [FAIL] ❌ Failed to delete pod %s/%s: %v\n", pod.Namespace, pod.Name, err)
	} else {
		fmt.Printf("   [SUCCESS] ✅ Deleted pod %s/%s. Controller is expected to recreate the Pod immediately.\n", pod.Namespace, pod.Name)
	}
}
