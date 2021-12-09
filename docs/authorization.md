---
nav_order: 3
---
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