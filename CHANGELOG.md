## Unreleased

FEATURES:

* New client: `NewClient` with API token (`WithAPIToken`) or basic authentication (`WithBasicAuth`), typed errors (`ErrorResponse` with per-field validation errors, matched with `errors.Is` against `ErrNotFound` and the other sentinels), `NewRequest` and `Do` for any endpoint, and no following of redirects
* New `Auth` service: `Token` returns the API token of a user
* New `ProductTypes` service: `List`, `ListAll` (`iter.Seq2`), `Get`, `GetByName`, `Create`, `Update`, `Patch`, `Delete`, `DeletePreview` and `CountProducts`
* `Nullable[T]`, with `Value` and `Null`, for partial updates that can set a field to null
* `Client.ServerVersion` returns the DefectDojo version from the served OpenAPI schema

NOTES:

* Tested against DefectDojo 3.4.304 and 3.5.103
