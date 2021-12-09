# ingress-dashboard

Automatic dashboard generation for Ingress objects.

Supports only v1/Ingress.

<img alt="image" src="https://user-images.githubusercontent.com/6597086/145249365-52035d08-469d-460e-b42c-e6af5d271e10.png">

# Installation

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

# Configuration

ingress-dashboard relies on annotations in
each [Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) object to configure dashboard.

## Annotation

All annotations are optional.

### Description

Annotation: `ingress-dashboard/description`

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

### Logo URL

Annotation: `ingress-dashboard/logo-url`

Defines custom logo URL for the ingress. It supports absolute URL (ex: `https://example.com/favicon.ico`) or relative
URL (ex: `/favicon.ico`). Relative URL should start from `/` and will be appended to the first endpoint in spec.

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

* it will get root page for each defined endpoint, parse it as HTML and use `href` attribute as logo url for tags `link`
  with attribute `rel` equal to
    * `apple-touch-icon`
    * `shortcut icon`
    * `icon`
    * `alternate icon`
* in case no logo URL found in HTML, ingress-dashboard will check `<url>/favicon.ico` URL and in case of 200 code
  response will use it as logo-url

### Title

Annotation: `ingress-dashboard/title`

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

### Hide

Annotation: `ingress-dashboard/hide`

Accepts `true` or `false` (default) string value.

If it set to `true`, ingress-dashboard will not render it in UI and will skip logo-url detection logic.

# Authorization

By-default, dashboard is not protected and available for everyone.

To restrict access ingress-dashboard supports following methods:

- basic authorization
- OIDC provider

Regardless of selected authorization ALWAYS use secured connection (ie: TLS/HTTPS)

## Basic authorization

[Basic authorization](https://datatracker.ietf.org/doc/html/rfc7617) assumes static username and password. It is not the
best option from security perspective, but good enough for internal usage or for testing.

For proper protection and for enterprise usage consider using OIDC.

To enable basic authorization, provide following environment variables:

* `AUTH=basic` - switch auth mode to HTTP basic
* `BASIC_USER=<your user name>` - desired user name (commonly `admin`)
* `BASIC_PASSWORD=<password>` - desired user password

Password is critical value so consider using [secrets](https://kubernetes.io/docs/concepts/configuration/secret/) to
store it.

## OIDC

[OIDC](https://openid.net/connect/) is industry standard for OAuth 2 Identity Providers (IDP) integration.

OIDC supported by many providers including (incomplete list):

- Auth0
- Google
- Microsoft
- Oracle
- Okta
- Keycloak
- and many more

To connect ingress-dashboard to IDP you need to obtain:

- issuer URL
- client ID and client secret

To enable OIDC authorization, provide following environment variables:

* `AUTH=oidc` - switch auth mode to OIDC
* `OIDC_ISSUER=<issuer-url>` - IDP URL (ex: for Keycloak it will be `https://<domain>/auth/realms/<realm>`)
* `CLIENT_ID=<client-id>` - client ID from IDP
* `CLIENT_SECRET=<password>` - client secret from IDP (sensitive information -
  use [secrets](https://kubernetes.io/docs/concepts/configuration/secret/) to store)
* `SERVER_URL=<public URL>` - (optional) URL of ingress-dashboard, used for redirects. If not set, dashboard will rely
  on Host header.