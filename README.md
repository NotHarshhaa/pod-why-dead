# pod-why-dead



> **One command. Full death story of any Kubernetes pod.**



Engineers waste 10–20 minutes every time a pod dies — running `kubectl describe`, `kubectl logs --previous`, `kubectl get events`, cross-referencing node pressure, checking resource limits. `pod-why-dead` does all of that in one shot and hands you a structured postmortem in seconds.



```bash

$ pod-why-dead -n production my-api-7f9d4b-xkzp2



 Pod Death Report ─────────────────────────────────────

  Pod        my-api-7f9d4b-xkzp2

  Namespace  production

  Node       ip-10-0-1-42.ec2.internal



 Cause: OOMKilled

  Container  api (exit code 137)

  Memory limit  512Mi

  Peak usage    509Mi (99.4% of limit)

  Killed at     2024-11-14 09:42:17 UTC



 Timeline

  09:41:03  Pod scheduled on ip-10-0-1-42

  09:41:09  Container started

  09:42:11  Memory usage crossed 90% threshold

  09:42:17  OOMKilled by kernel



 Node Pressure at time of death

  Memory pressure  true

  Node available   48Mi of 8Gi



 Last 20 log lines (before death)

  [09:42:15] WARN  Memory usage high, processing batch of 4200 records

  [09:42:16] ERROR Failed to allocate buffer: out of memory

  [09:42:17] FATAL process killed



 Recommendation

  Increase memory limit above 512Mi or investigate batch size in

  the record processing loop. Node was already under memory pressure

  before scheduling — consider a PodDisruptionBudget or node affinity rule.

──────────────────────────────────────────────────────

```



---



## Why this exists



When a pod dies in Kubernetes, the information you need is scattered across five different commands. Nobody has time for that during an incident.



| What you need | Without pod-why-dead | With pod-why-dead |
|---|---|---|
| Exit code + reason | `kubectl describe pod` | Instant |
| Last logs | `kubectl logs --previous` | Instant |
| Node conditions | `kubectl describe node` | Instant |
| Resource usage | `kubectl top pod` (if still alive) | Reconstructed |
| Event timeline | `kubectl get events --field-selector` | Instant |



---



## Install



### Quick Install (Recommended)

**Linux / macOS:**
```bash
curl -sSL https://raw.githubusercontent.com/NotHarshhaa/pod-why-dead/main/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/NotHarshhaa/pod-why-dead/main/install.ps1 | iex
```

---

### Package Managers

**Homebrew (macOS / Linux):**
```bash
brew install NotHarshhaa/tap/pod-why-dead
```

**Scoop (Windows):**
```powershell
scoop bucket add NotHarshhaa https://github.com/NotHarshhaa/scoop-bucket
scoop install pod-why-dead
```

**Go install:**
```bash
go install github.com/NotHarshhaa/pod-why-dead@latest
```

**kubectl plugin (krew - manual install):**
```bash
kubectl krew install --manifest-url=https://raw.githubusercontent.com/NotHarshhaa/pod-why-dead/main/krew/pod-why-dead.yaml --manifest-file=pod-why-dead.yaml pod-why-dead
kubectl pod-why-dead -n production my-pod-name
```

---

**Docker:**
```bash
# Pull the image
docker pull ghcr.io/notharshhaa/pod-why-dead:latest

# Run with kubeconfig mounted
docker run --rm -v ~/.kube/config:/root/.kube/config ghcr.io/notharshhaa/pod-why-dead:latest -n production my-pod
```

---

### Manual Binary Download

Download from [Releases](https://github.com/NotHarshhaa/pod-why-dead/releases) and place in your `$PATH`.

**Linux (amd64):**
```bash
curl -sSL https://github.com/NotHarshhaa/pod-why-dead/releases/latest/download/pod-why-dead_linux_amd64.tar.gz | tar -xz
sudo mv pod-why-dead /usr/local/bin/
```

**macOS (Apple Silicon):**
```bash
curl -sSL https://github.com/NotHarshhaa/pod-why-dead/releases/latest/download/pod-why-dead_darwin_arm64.tar.gz | tar -xz
sudo mv pod-why-dead /usr/local/bin/
```

**Windows:**
```powershell
Invoke-WebRequest -Uri "https://github.com/NotHarshhaa/pod-why-dead/releases/latest/download/pod-why-dead_windows_amd64.zip" -OutFile "pod-why-dead.zip"
Expand-Archive -Path "pod-why-dead.zip" -DestinationPath "."
```

---

### Architecture Support

- **Linux**: amd64, arm64
- **macOS**: amd64 (Intel), arm64 (Apple Silicon)
- **Windows**: amd64



---



## Usage



### Basic

```bash

# Inspect a specific pod

pod-why-dead -n <namespace> <pod-name>



# Also works as kubectl plugin

kubectl why-dead -n <namespace> <pod-name>

```



### Flags



| Flag | Description | Default |

|---|---|---|

| `-n, --namespace` | Kubernetes namespace | `default` |

| `--context` | kubeconfig context to use | current context |

| `--log-lines` | Number of previous log lines to show | `20` |

| `--output` | Output format: `text`, `json`, `markdown` | `text` |

| `--no-recommendations` | Skip the recommendations section | `false` |

| `--since` | Look at pods that died within duration (e.g. `2h`, `30m`) | `24h` |



### Output formats



```bash

# Default pretty-printed terminal output

pod-why-dead -n production my-pod



# JSON (pipe to jq, store in incident log)

pod-why-dead -n production my-pod --output json | jq .cause



# Markdown (paste into incident report / Notion / Confluence)

pod-why-dead -n production my-pod --output markdown > incident.md

```



### List all recently dead pods in a namespace

```bash

pod-why-dead -n production --list --since 1h

```



```

 Recently Dead Pods (last 1h) — namespace: production

  my-api-7f9d4b-xkzp2      OOMKilled     09:42:17

  worker-6c8f9d-mnbv1       CrashLoopBack 09:51:03

  scheduler-5d7b2a-pqrs8    Error (1)     10:03:44

```



---



## What it checks


`pod-why-dead` reconstructs the full picture from what Kubernetes still knows after a pod is gone:


- **Exit code + reason** — OOMKilled, Error, Evicted, CrashLoopBackOff with detailed explanations

- **Exit code deep dive** — detailed explanations for common exit codes (1, 127, 128+signal, etc.)

- **Previous container logs** — last N lines before death

- **Full event timeline** — from scheduling to termination

- **Node conditions at death** — MemoryPressure, DiskPressure, PIDPressure

- **Node information** — kernel version, OS, container runtime, taints, labels

- **Scheduling constraints** — node selectors, tolerations, affinity rules

- **Persistent Volume Claims** — PVC status, capacity, storage class

- **Resource quota analysis** — namespace resource quota usage

- **Resource limit vs peak usage** — how close were you to the limit?

- **Restart history** — first death or 47th?

- **Liveness / readiness probe failures** — did a probe kill it?

- **Recommendations** — concrete next steps based on the cause

- **Suggested kubectl commands** — ready-to-run commands for debugging



---


## Death causes handled



| Cause | What pod-why-dead shows |

|---|---|

| `OOMKilled` | Memory limit, peak usage, node memory pressure |

| `CrashLoopBackOff` | Restart count, backoff duration, repeated error pattern |

| `Error (exit code N)` | Exit code, last log lines, container command |

| `Evicted` | Eviction reason, node resource that triggered it |

| `Liveness probe failed` | Probe config, failure count, what the probe hit |

| `ImagePullBackOff` | Image name, registry error message |

| `Pending → never started` | Scheduling failure reason (insufficient CPU/memory/taints) |



---



## Requirements



- Go 1.21+

- A valid `kubeconfig` (same as `kubectl`)

- RBAC: `get` and `list` on `pods`, `pods/log`, `events`, `nodes`, `persistentvolumeclaims`, `resourcequotas`



### Minimal RBAC for read-only use

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: pod-why-dead-reader
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log", "events", "nodes", "persistentvolumeclaims", "resourcequotas"]
    verbs: ["get", "list"]
```



---



## Roadmap



- [ ] `--watch` mode: continuously monitor a namespace and auto-report on new deaths

- [ ] Slack / PagerDuty webhook output

- [ ] Correlate with HPA / VPA events

- [ ] Historical mode using past events from Loki / OpenSearch

- [ ] GitHub Actions integration — auto-comment on PRs when staging pods die



---



## Contributing



PRs are welcome. Please open an issue first for anything beyond small fixes.



```bash

git clone https://github.com/NotHarshhaa/pod-why-dead

cd pod-why-dead

go mod tidy

go run . -n default <your-pod-name>

```



---



## License



MIT © [NotHarshhaa](https://github.com/NotHarshhaa)

