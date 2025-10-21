k8s-healer

A powerful and reliable command-line interface (CLI) tool written in Go that watches specified Kubernetes namespaces for unhealthy Pods and automatically performs a healing action.

🌟 Features

Real-time Monitoring: Uses the efficient client-go Informer pattern to watch the Kubernetes API server for Pod status changes, avoiding heavy polling.

Targeted Healing: Automatically detects and deletes Pods exhibiting persistent failures (e.g., CrashLoopBackOff) to force a clean recreation by the managing controller (Deployment, StatefulSet, etc.).

Flexible Namespace Selection: Supports watching multiple, specific namespaces or simple wildcard patterns (app-*) for dynamic environment targeting.

Graceful Shutdown: Handles OS signals (Ctrl+C) cleanly to stop all internal watchers without disruption.

🩹 Healing Strategy

The tool implements the standard Kubernetes remediation strategy for managed resources:

Detection: If a Pod's container status indicates CrashLoopBackOff and its RestartCount exceeds a threshold (default: 3), it is flagged as persistently unhealthy.

Deletion: The tool sends an API request to delete the unhealthy Pod.

Reconciliation: The managing ReplicaSet or Controller immediately notices the missing replica and spins up a brand-new Pod, effectively "healing" the service by replacing the faulty instance.

🚀 Usage

Build

# 1. Initialize the project (if you haven't already)
go mod tidy

# 2. Build the executable
go build -o k8s-healer ./cmd/main.go


Run

The tool authenticates using your local Kubeconfig file by default.

Flag

Description

Example

-n, --namespaces

Comma-separated list of namespaces to watch. Supports simple wildcards (*).

-n "prod-*,staging"

-k, --kubeconfig

Path to a specific kubeconfig file.

-k ~/.kube/config

Examples:

# Watch all namespaces (default behavior when -n is empty)
./k8s-healer

# Watch specific, comma-separated namespaces
./k8s-healer -n frontend,backend

# Watch namespaces matching a wildcard pattern
./k8s-healer -n 'app-*-dev,tools-*'

# Use an alternate kubeconfig file
./k8s-healer -k /etc/k8s/admin.conf -n default

## License

[MIT](./LICENSE) © 2025 Bruno Maio%  