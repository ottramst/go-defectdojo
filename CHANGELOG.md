## v0.0.1

FEATURES:

* New client: `NewClient` with API token (`WithAPIToken`) or basic authentication (`WithBasicAuth`), typed errors (`ErrorResponse` with per-field validation errors, matched with `errors.Is` against `ErrNotFound` and the other sentinels), `NewRequest` and `Do` for any endpoint, and no following of redirects. `Response.Deprecated` returns the `X-Deprecated` header of endpoints that DefectDojo will remove ([#4](https://github.com/ottramst/go-defectdojo/issues/4))
* New `Auth` service: `Token` returns the API token of a user ([#6](https://github.com/ottramst/go-defectdojo/issues/6))
* New `ProductTypes` service: `List`, `ListAll` (`iter.Seq2`), `Get`, `GetByName`, `Create`, `Update`, `Patch`, `Delete`, `DeletePreview` and `CountProducts` ([#5](https://github.com/ottramst/go-defectdojo/issues/5))
* `Nullable[T]`, with `Value` and `Null`, for partial updates that can set a field to null ([#4](https://github.com/ottramst/go-defectdojo/issues/4))
* `Client.ServerVersion` returns the DefectDojo version from the served OpenAPI schema ([#6](https://github.com/ottramst/go-defectdojo/issues/6))

NOTES:

* First preview release, for terraform-provider-defectdojo v0.0.1. Products, engagements and the other services of the v0.1.0 milestone follow
* Tested against DefectDojo 3.4.304 and 3.5.103. A contract test checks the client's types against the OpenAPI schema of each supported version ([#7](https://github.com/ottramst/go-defectdojo/issues/7))
