package util

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
)

// DefaultRestartThreshold is the number of times a container must restart
// before we consider the Pod persistently unhealthy and eligible for healing.
const DefaultRestartThreshold = 3

// IsUnhealthy checks if a Pod exhibits signs of persistent failure that requires healing.
// This function implements the core criteria: currently only CrashLoopBackOff.
func IsUnhealthy(pod *v1.Pod) bool {
	// A Pod is considered unhealthy if any of its containers are in CrashLoopBackOff
	// and have exceeded the restart threshold.
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil && status.State.Waiting.Reason == "CrashLoopBackOff" {
			if status.RestartCount >= DefaultRestartThreshold {
				fmt.Printf("   [Check] 🚨 Pod %s/%s failed check: CrashLoopBackOff (Restarts: %d).\n",
					pod.Namespace, pod.Name, status.RestartCount)
				return true
			}
		}
	}

	// Add checks for other failure phases like PodFailed, or ImagePullBackOff here if needed.

	return false
}

// GetHealReason retrieves the specific reason for the healing action.
func GetHealReason(pod *v1.Pod) string {
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil && status.State.Waiting.Reason == "CrashLoopBackOff" {
			if status.RestartCount >= DefaultRestartThreshold {
				return fmt.Sprintf("Persistent CrashLoopBackOff (Restarts: %d)", status.RestartCount)
			}
		}
	}
	return "Unspecified Failure"
}
