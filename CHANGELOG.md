## Unreleased

FEATURES:

* New `SLAConfigurations` service: `List`, `ListAll`, `Get`, `GetByName`, `Create`, `Update`, `Patch`, `Delete` and `DeletePreview`, and `IsSLARecalculating`, which recognises DefectDojo's 400 for an SLA change made while the SLA dates of an earlier change are still being recalculated ([#13](https://github.com/ottramst/go-defectdojo/issues/13))
* New `Users` service: `List`, `ListAll`, `Get`, `GetByUsername`, `Create` (with `UserCreate`, which adds the initial password), `Update`, `Patch`, `Delete`, `DeletePreview` and `ResetAPIToken`. `User` gains `TokenLastReset`, `TokenExpiry` and `PasswordLastReset` ([#14](https://github.com/ottramst/go-defectdojo/issues/14))
* New `ConfigurationPermissions` service: `List`, `ListAll`, `Get` and `GetByCodename`, to find the IDs of configuration permissions, which differ between DefectDojo installations ([#43](https://github.com/ottramst/go-defectdojo/issues/43))
* New `NotificationWebhooks` service: `List`, `ListAll`, `Get`, `GetByName`, `Create`, `Update`, `Patch`, `Delete` and `DeletePreview`, and the `NotificationWebhookStatus*` constants ([#16](https://github.com/ottramst/go-defectdojo/issues/16))
* New `Notifications` service for notification settings (`NotificationSettings`): `List`, `ListAll`, `Get`, `GetByScope`, `Create`, `Update`, `Patch`, `Delete` and `DeletePreview`, with the `NotificationChannel*` constants and `NotificationChannels` ([#15](https://github.com/ottramst/go-defectdojo/issues/15))

ENHANCEMENTS:

* `ErrorResponse.Message` holds the text of error bodies that are a bare JSON string or list of strings, such as DefectDojo's "Users may not delete themselves", instead of the raw JSON ([#14](https://github.com/ottramst/go-defectdojo/issues/14))

NOTES:

* `SLAConfigurations.Create` and `SLAConfigurations.Update` send every writable field, so `EnforceCritical`, `EnforceHigh`, `EnforceMedium` and `EnforceLow`, which DefectDojo defaults to `true`, are `false` unless they are set. Days below 1, which DefectDojo's API accepts but its UI rejects, are refused before sending ([#13](https://github.com/ottramst/go-defectdojo/issues/13))
* `Notifications`: DefectDojo creates the system-wide settings and each user's personal settings itself, so find them with `GetByScope` rather than creating them. `Create` and `Update` leave nil events out, which gives them DefectDojo's default (`alert`), and send a nil `ScanAddedEmpty` as no channel; `Update` always sends the scope, which DefectDojo resets otherwise, and can't update the template: use `Patch`. `ScanAddedEmpty` holds at most one channel through the API ([#15](https://github.com/ottramst/go-defectdojo/issues/15))
* `Users.Create` sends every writable field, so `IsActive`, which DefectDojo defaults to `true`, is `false` unless it is set. `Users.Create` and `Users.Update` leave empty `ConfigurationPermissions` out, which keeps the user's permissions, so that a user read with `Users.Get` can be updated safely: DefectDojo answers an empty list by removing all the user's Django permissions, not only the configuration permissions. Remove them with `Users.Patch`. DefectDojo's API can't change a password once the user exists ([#14](https://github.com/ottramst/go-defectdojo/issues/14))

## v0.1.0

FEATURES:

* New `Products` service: `List`, `ListAll`, `Get`, `GetByName`, `Create`, `Update`, `Patch`, `Delete`, `DeletePreview` and `CountEngagements` ([#9](https://github.com/ottramst/go-defectdojo/issues/9))
* New `Engagements` service: `List`, `ListAll`, `Get`, `Create`, `Update`, `Patch`, `Delete`, `DeletePreview`, `Close`, `Reopen` and `CountTests` ([#10](https://github.com/ottramst/go-defectdojo/issues/10))
* `Date`, a calendar date sent as `2006-01-02`, with `DateOf` and `ParseDate` ([#10](https://github.com/ottramst/go-defectdojo/issues/10))
* New `UserProfile` service: `Get` returns the user the client authenticates as, with its contact information ([#11](https://github.com/ottramst/go-defectdojo/issues/11))

NOTES:

* `Products.Create` and `Products.Update` send every writable field, so `EnableFullRiskAcceptance`, which DefectDojo defaults to `true`, is `false` unless it is set. Zero values of nullable fields (`Platform`, `ProductManager` and the like) are sent as `null`, a zero `SLAConfiguration` and nil `UserRecords` and `Revenue` are left out (DefectDojo's default on create, the current value on update), and `ProdNumericGrade`, which DefectDojo calculates, is only written by `Products.Patch` ([#9](https://github.com/ottramst/go-defectdojo/issues/9))
* `Engagements.Create` and `Engagements.Update` send `ThreatModel`, `APITest`, `PenTest` and `CheckList`, which DefectDojo defaults to `true`, so they are `false` unless they are set; an empty `Status` or `EngagementType` is left out (DefectDojo's default on create, the current value on update), and nil `Tags` too, so that a new engagement inherits its product's tags and `Update` keeps the current tags; a non-nil empty `Tags` removes them all, inherited ones included. An engagement's name can't be cleared: DefectDojo names it after its target start ([#10](https://github.com/ottramst/go-defectdojo/issues/10))
* `Products.GetByName` matches names case-sensitively: DefectDojo's `name_exact` filter ignores case, and product names that differ only in case can coexist ([#9](https://github.com/ottramst/go-defectdojo/issues/9))
* Completes the v0.1.0 milestone (products, engagements and the user profile), for terraform-provider-defectdojo v0.1.0. Tested against DefectDojo 3.4.304 and 3.5.103

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
