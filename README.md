# 🩺 k8s-healer

A powerful and reliable **command-line interface (CLI)** tool written in
Go that watches specified Kubernetes namespaces for unhealthy Pods and
automatically performs a healing action --- safely and intelligently.

------------------------------------------------------------------------

## 🌟 Overview

**k8s-healer** continuously monitors Kubernetes Pods and automatically
heals those stuck in persistent failure states (like
`CrashLoopBackOff`).\
It does this by deleting the faulty Pod, allowing the managing
controller (e.g., Deployment, StatefulSet) to recreate a fresh, healthy
instance.

The tool is designed to be lightweight, safe, and extensible ---
suitable for both local development and production environments.

------------------------------------------------------------------------

## ⚙️ Key Features

-   **Real-time Monitoring:** Efficiently watches Pod status updates
    using Kubernetes Informers (no polling).\
-   **Targeted Healing:** Automatically deletes Pods that exceed a
    restart threshold and are stuck in states like `CrashLoopBackOff`.\
-   **Healing Cooldown (🆕):** Prevents repeated healing loops by
    skipping Pods recently healed within a configurable cooldown period
    (default: **10 minutes**).\
-   **Flexible Namespace Selection:** Supports multiple namespaces and
    wildcard patterns (e.g. `app-*`, `prod-*`).\
-   **Graceful Shutdown:** Handles OS signals cleanly (`Ctrl+C`,
    `SIGTERM`) for safe exits.\
-   **Pluggable Architecture:** Easy to extend with new failure
    detection logic or healing strategies.

------------------------------------------------------------------------

## 🩹 Healing Strategy

### Default Workflow

1.  **Detection:**\
    The tool checks if a Pod has containers in `CrashLoopBackOff` or
    other failure states (e.g., `ImagePullBackOff`).\
    If a container's `RestartCount` exceeds a configurable threshold
    (default: `3`), the Pod is marked unhealthy.

2.  **Cooldown Check (🆕):**\
    Before deleting, k8s-healer verifies whether the Pod was healed
    recently.\
    If the same Pod was healed within the **cooldown window** (default:
    `10m`), it is skipped to avoid endless delete/recreate loops.

3.  **Deletion:**\
    If eligible, the Pod is deleted via the Kubernetes API.\
    Its managing controller detects the deletion and immediately spins
    up a replacement Pod.

4.  **Reconciliation:**\
    The new Pod is scheduled and started fresh --- effectively
    self-healing the workload.

------------------------------------------------------------------------

## 🚀 Usage

### 🏗️ Build

``` bash
# 1. Initialize dependencies
go mod tidy

# 2. Build the executable
go build -o k8s-healer ./cmd/main.go
```

### ▶️ Run

By default, the tool authenticates using your local **Kubeconfig** file
(e.g. `~/.kube/config`).

  ------------------------------------------------------------------------------
  Flag                 Description                       Example
  -------------------- --------------------------------- -----------------------
  `-n, --namespaces`   Comma-separated list of           `-n "prod-*,staging"`
                       namespaces to watch. Supports     
                       wildcards (`*`).                  

  `-k, --kubeconfig`   Path to a specific kubeconfig     `-k ~/.kube/config`
                       file.                             

  `--heal-cooldown`    Minimum duration between healing  `--heal-cooldown 5m`
                       the same Pod. Default: `10m`.     
  ------------------------------------------------------------------------------

------------------------------------------------------------------------

### 💡 Examples

``` bash
# Watch all namespaces (default)
./k8s-healer

# Watch specific namespaces
./k8s-healer -n frontend,backend

# Watch namespaces matching wildcard patterns
./k8s-healer -n 'app-*-dev,tools-*'

# Use an alternate kubeconfig
./k8s-healer -k /etc/k8s/admin.conf -n default

# Apply a custom healing cooldown period (5 minutes)
./k8s-healer --heal-cooldown 5m -n production
```

------------------------------------------------------------------------

## 🔄 Example Output

    [Check] 🚨 Pod prod/api-7d8f9 failed check: CrashLoopBackOff (Restarts: 4).
    !!! HEALING ACTION REQUIRED !!!
        Pod: prod/api-7d8f9
        Reason: Persistent CrashLoopBackOff (Restarts: 4)
    [SUCCESS] ✅ Deleted pod prod/api-7d8f9. Controller is expected to recreate the Pod immediately.
    !!! HEALING ACTION COMPLETE !!!

    [SKIP] ⏳ Pod prod/api-7d8f9 was healed 120 seconds ago — skipping re-heal.

------------------------------------------------------------------------

## 🧠 Why the Cooldown Matters

Without a cooldown mechanism, a continuously crashing Pod could cause a
"healing storm" --- a rapid delete/recreate cycle that strains the
control plane and hides deeper issues (like bad configs or image pull
errors).

The **healing cooldown** ensures each Pod has a grace period to
stabilize before another healing attempt.

------------------------------------------------------------------------

## 🧰 Extensibility

You can easily extend **k8s-healer** by: - Adding new unhealthy
conditions in `util.IsUnhealthy()`. - Adjusting thresholds or strategies
in `util.DefaultRestartThreshold`. - Integrating logging or Prometheus
metrics for observability.

------------------------------------------------------------------------

## 📄 License

[MIT](./LICENSE) © 2025 Bruno Maio