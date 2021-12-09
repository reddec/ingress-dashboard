---
parent: configuration
---

## Static source

For the external resource (without Ingress definitions) it is possible to
define static dashboard definitions.

Static definitions always placed before dynamic. No automatic logo URL detection available.

To enable static source define environment `STATIC_SOURCE=/path/to/source`, where source
could be directory or single file.

Directories are scanned recursively for each file with extension `.yml`, `.yaml`, or `.json`.
YAML documents may contain multiple definitions.

Support fields:

* `name` - resource label
* `namespace` - (optional) resource namespace, used in `from`
* `description` - (optional) resource description
* `hide` - (optional) mark resource as hidden or not. Default is `false`
* `urls` - list of urls
* `logo_url` - (optional) URL for log


Example:

```yaml
---
name: Some site
namespace: external links
urls:
  - https://example.com
---
name: Google
logo_url: https://www.google.ru/favicon.ico
description: |
  Well-known search engine
urls:
  - https://google.com
```

Example usage in kubernetes with [config map](https://kubernetes.io/docs/concepts/configuration/configmap/):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: static
  namespace: ingress-dashboard
data:
  static.yaml: |
    ---
    name: Some site
    namespace: external links
    urls:
      - https://example.com
    ---
    name: Google
    logo_url: https://www.google.ru/favicon.ico
    description: |
      Well-known search engine
    urls:
      - https://google.com
```

> Hint: you may use Kustomize [config map generator](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/#configmapgenerator) to simplify the process

And update deployment

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: "dashboard"
  namespace: "ingress-dashboard"
spec:
  # ...
  template:
    # ...
    spec:
      # ...
      containers:
      - name: "dashboard"
        # ...
        env:
        # ... 
        - name: STATIC_SOURCE
          value: /static
        # ...
        volumeMounts:
        - name: config-volume
          mountPath: /static
      # ...
      volumes:
        - name: config-volume
          configMap:
            name: static
```
