---
nav_order: 1
---
# Installation

Docker image supports amd64 and arm64 architectures.

Kubernetes with RBAC

    curl -L https://github.com/reddec/ingress-dashboard/releases/latest/download/ingress-dashboard.yaml | \
    kubectl apply -f -

Optionally use Ingress to open access to dashboard. Read [authorization](#authorization) how to secure access.

**Example**

```yaml
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dashboard
  namespace: ingress-dashboard
  annotations:
    kubernetes.io/ingress.class: "nginx" # may vary
    ingress-dashboard/title: "Dashboard"
    ingress-dashboard/description: "Dashboard of ingress resources"
    ingress-dashboard/logo-url: /favicon.ico
spec:
  rules:
    - host: dashboard.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: dashboard
                port:
                  number: 8080
```
