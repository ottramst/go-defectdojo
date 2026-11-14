## Unreleased

FEATURES:

* New `Products` service: `List`, `ListAll`, `Get`, `GetByName`, `Create`, `Update`, `Patch`, `Delete`, `DeletePreview` and `CountEngagements` ([#9](https://github.com/ottramst/go-defectdojo/issues/9))

NOTES:

* `Products.Create` and `Products.Update` send every writable field, so `EnableFullRiskAcceptance`, which DefectDojo defaults to `true`, is `false` unless it is set. Zero values of nullable fields (`Platform`, `ProductManager` and the like) are sent as `null`, a zero `SLAConfiguration` and nil `UserRecords` and `Revenue` are left out (DefectDojo's default on create, the current value on update), and `ProdNumericGrade`, which DefectDojo calculates, is only written by `Products.Patch` ([#9](https://github.com/ottramst/go-defectdojo/issues/9))
* `Products.GetByName` matches names case-sensitively: DefectDojo's `name_exact` filter ignores case, and product names that differ only in case can coexist ([#9](https://github.com/ottramst/go-defectdojo/issues/9))

## v0.0.1

FEATURES:

* New client: `NewClient` with API token (`WithAPIToken`) or basic authentication (`WithBasicAuth`), `WithHTTPClient` and `WithUserAgent`, per-request `WithHeader` and `WithQueryParam`, typed errors (`ErrorResponse` with per-field validation errors, matched with `errors.Is` against `ErrNotFound` and the other sentinels), `CheckResponse`, `NewRequest` and `Do` for any endpoint, and no following of redirects. `Response.Deprecated` returns the `X-Deprecated` header of endpoints that DefectDojo will remove ([#4](https://github.com/ottramst/go-defectdojo/issues/4))
* New `Auth` service: `Token` returns the API token of a user ([#6](https://github.com/ottramst/go-defectdojo/issues/6))
* New `ProductTypes` service: `List`, `ListAll` (`iter.Seq2`), `Get`, `GetByName`, `Create`, `Update`, `Patch`, `Delete`, `DeletePreview` and `CountProducts` ([#5](https://github.com/ottramst/go-defectdojo/issues/5))
* `Nullable[T]`, with `Value` and `Null`, for partial updates that can set a field to null ([#4](https://github.com/ottramst/go-defectdojo/issues/4))
* `Client.ServerVersion` returns the DefectDojo version from the served OpenAPI schema ([#6](https://github.com/ottramst/go-defectdojo/issues/6))

NOTES:

* First preview release, for terraform-provider-defectdojo v0.0.1. Products, engagements and the other services of the v0.1.0 milestone follow
* Tested against DefectDojo 3.4.304 and 3.5.103. A contract test checks the client's types against the OpenAPI schema of each supported version ([#7](https://github.com/ottramst/go-defectdojo/issues/7))
