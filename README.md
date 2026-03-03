# KubeSentinel

Kubernetes Security Scanner - Real vulnerability and misconfiguration scanner for K8s clusters.

## Installation

```bash
go install github.com/dablon/kubesentinel@latest
```

Or build from source:

```bash
git clone https://github.com/dablon/kubesentinel.git
cd kubesentinel
go install ./cmd
```

## Usage

```bash
# Scan all namespaces
kubesentinel scan

# Scan specific namespace
kubesentinel scan -n production

# Output JSON
kubesentinel scan -o json

# Filter severity
kubesentinel scan -s critical
kubesentinel scan -s high
```

## Example Output

```
=== KubeSentinel Security Scan ===
Cluster: demo-cluster
Time: 2026-03-03T05:00:00Z

--- Summary ---
Total Issues: 8
  Critical: 1
  High: 4
  Medium: 2
  Low: 1
Scanned Pods: 3

--- Issues ---
🔴 [critical] Container nginx runs in privileged mode
   Namespace: production
   Resource: pod/nginx-prod
   Fix: Set privileged: false

🟠 [high] Container nginx runs as root
   Namespace: production
   Resource: pod/nginx-prod
   Fix: Set runAsNonRoot: true and runAsUser > 0
```

## Features

- Pod security context analysis
- RBAC auditing
- Capability detection
- Resource limit checking
- Host namespace access detection
- Multiple output formats (text, JSON)

## License

MIT
