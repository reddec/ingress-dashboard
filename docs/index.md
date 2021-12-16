![k-id](https://user-images.githubusercontent.com/6597086/145367873-ae85fba7-d3aa-47ba-8100-1ce6518aa463.png)

Automatic dashboard generation for Ingress objects.

Features:

* No JS
* Supports OIDC (Keycloak, Google, Okta, ...) and Basic authorization
* Automatic discovery of Ingress objects, configurable by annotations
* Supports static configuration (in addition to Ingress objects)
* Multiarch docker images: for amd64 and for arm64
* Automatic even-based updates

Limitations:

* Supports only v1/Ingress kind.
* Doesn't support Ingress Reference kind, only Service type
* Doesn't support DefaultBackend (I have no idea which URL to generate for it)
* Hosts number per Ingress calculated each Ingress update or after refresh (30s by default)

![image](https://user-images.githubusercontent.com/6597086/146317711-575b7be9-7fa9-47a4-90ee-5328393f4adc.png)
