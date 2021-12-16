---
title: Developing
nav_order: 999
---

# Developing KID

For local development environment

Requirements:

- go 1.17+ (the project aims to focus on the latest Go version)
- configured privileged access to the cluster (ie: kubeconfig file). For test purpose you may use something like minikube
- [golangci-lint](https://golangci-lint.run)

Supported OS for developing:

- Linux (any)
- MacOS (including Apple Silicon)
- Windows? (not tested)


Local start

    go run -v ./cmd/ingress-dashboard/main.go --kubeconfig ~/.kube/config

Navigate to http://127.0.0.1:8080