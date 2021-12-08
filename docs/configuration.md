# Configuration

ingress-dashboard relies on annotations in each [Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) object to configure dashboard.

## Annotation

All annotations are optional.

### `ingress-dashboard/description`

Defines custom description for the ingress. If not defined, no description will be shown.

Example:

```yaml
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: demo
  annotations:
    ingress-dashboard/description: |
      This is demo service
spec:
  rules:
  - host: demo.example.com
    http:
      paths:
      - path: /foo/
        pathType: Prefix
        backend:
          service:
            name: my-service
            port:
              number: 8080
```

### `ingress-dashboard/logo-url`

Defines custom logo URL for the ingress. It supports absolute URL (ex: `https://example.com/favicon.ico`) or 
relative URL (ex: `/favicon.ico`). Relative URL should start from `/` and will be appended to the first endpoint in spec.

Example:

```yaml
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: demo
  annotations:
    ingress-dashboard/logo-url: "/favicon.ico"
spec:
  rules:
  - host: demo.example.com
    http:
      paths:
      - path: /foo/
        pathType: Prefix
        backend:
          service:
            name: my-service
            port:
              number: 8080
```

Logo URL will point to `http://demo.example.com/foo/favicon.ico`

If logo URL not defined, ingress-dashboard will try to detect it automatically:

* it will get root page for each defined endpoint, parse it as HTML and use `href` attribute as logo url for tags `link`with attribute `rel` equal to 
  * `apple-touch-icon`
  * `shortcut icon`
  * `icon`
  * `alternate icon`
* in case no logo URL found in HTML, ingress-dashboard will check `<url>/favicon.ico` URL and in case of 200 code response will use it as logo-url

## `ingress-dashboard/title`

Defines custom service title. If not defined - ingress name will be used.

```yaml
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: demo
  annotations:
    ingress-dashboard/title: Demo App
spec:
  rules:
  - host: demo.example.com
    http:
      paths:
      - path: /foo/
        pathType: Prefix
        backend:
          service:
            name: my-service
            port:
              number: 8080
```
