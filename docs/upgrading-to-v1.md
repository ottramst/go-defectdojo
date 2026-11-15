# Upgrading to v1

v1.0.0 is the first stable release of go-defectdojo. From v1.0.0, the module follows
[semantic versioning](https://semver.org/): breaking changes need a new major version. See
[Stability](../README.md#stability) for what that covers.

v1.0.0 reworks the API once, so that it can stay stable. This page lists every change from v0.4.0
that can break your code, with what to use instead.

## Client core

| v0.4.0 | v1.0.0 |
| --- | --- |
| `Response` embeds `*http.Response` | `Response{StatusCode, Status, Header}`. The body was already read before. For other details of the HTTP exchange, such as the TLS state, use a custom `http.RoundTripper` in `WithHTTPClient`. |
| `ErrorResponse.Response` | `ErrorResponse.Method`, `URL` (without credentials) and `Header`. The error no longer keeps the request, whose headers hold the API token. |
| `CheckResponse(resp)` | Removed. `Client.Do` returns the `*ErrorResponse`. |
| `version, err := client.ServerVersion(ctx)` | `version, resp, err := client.ServerVersion(ctx, nil)`; `resp` is nil when the version is cached. |
| Lookups that find several objects return an error that matches no sentinel | The error matches `ErrAmbiguous`: `CICDInfrastructure.GetByName`, `DevelopmentEnvironments.GetByName`, `JiraInstances.GetByName` and `Notifications.GetByScope`. |

## Method signatures

Every method that calls DefectDojo takes a nil-able options struct before the variadic request
options. Pass `nil` unless you need an option:

| v0.4.0 | v1.0.0 |
| --- | --- |
| `client.Products.Get(ctx, id)` | `client.Products.Get(ctx, id, nil)` |
| `client.ProductTypes.Create(ctx, pt)` | `client.ProductTypes.Create(ctx, pt, nil)` |
| `client.Engagements.Patch(ctx, id, patch)` | `client.Engagements.Patch(ctx, id, patch, nil)` |
| `client.Users.GetByUsername(ctx, name)` | `client.Users.GetByUsername(ctx, name, nil)` |
| `client.Auth.Token(ctx, user, password)` | `client.Auth.Token(ctx, user, password, nil)` |
| `client.Products.DeletePreview(ctx, id, &defectdojo.ListOptions{Limit: 50})` | `client.Products.DeletePreview(ctx, id, &defectdojo.DeletePreviewProductOptions{ListOptions: defectdojo.ListOptions{Limit: 50}})` |
| `client.Announcements.List(ctx, &defectdojo.ListOptions{…})` | `client.Announcements.List(ctx, &defectdojo.ListAnnouncementsOptions{…})`, which also filters by `Message`, `Style` and `Dismissable` |
| `client.ProductPlatforms.List(ctx, &defectdojo.ListOptions{…})`, also lifecycles and origins | `client.ProductPlatforms.List(ctx, &defectdojo.ListProductAttributesOptions{…})` |
| `client.Notifications.GetByScope(ctx, user, product, template)` | `client.Notifications.GetByScope(ctx, defectdojo.NotificationScope{User: user, Product: product, Template: template}, nil)` |
| `client.CICDInfrastructure.GetByName(ctx, name, typ)` | `client.CICDInfrastructure.GetByName(ctx, name, &defectdojo.GetCICDInfrastructureByNameOptions{InfrastructureType: typ})`, or `nil` for any type |

The options types are named `<Method><Noun>Options`, after the object rather than the service:
`GetProductOptions`, `CreateEngagementOptions`, `GetUserByUsernameOptions`,
`CountProductTypeProductsOptions`, `GetNotificationSettingsOptions`. A method that names its noun takes
`<Method>Options` (`CurrentAnnouncementOptions`, `TokenOptions`). `ListAll` shares the options of `List`.

## Renamed types

| v0.4.0 | v1.0.0 |
| --- | --- |
| `ProductTypeList` with `ProductTypes`, and the other `<X>List` types | `ListProductTypesResult` with `Results`, and so on for every service |
| `CICDInfrastructureList.CICDInfrastructure` | `ListCICDInfrastructureResult.Results` |
| `NotificationSettingsList` | `ListNotificationSettingsResult` |
| `ListNotificationsOptions` | `ListNotificationSettingsOptions` |
| `DeletePreviewList.Objects` | `DeletePreviewResult.Results` |
| `UserCreate` | `CreateUserRequest` |
| `JiraInstanceWrite` | `CreateJiraInstanceRequest`, also the body of `Update` |
| `NotificationChannels` | Removed: `[]NotificationChannel` |

## Result shapes

| v0.4.0 | v1.0.0 |
| --- | --- |
| `page.Results` is `[]*T` | `[]T`. Use `&page.Results[i]` for a pointer. The `ListAll` iterators still yield `*T`. |
| `NotificationSettings.ProductAdded` and the other events are `[]string`, `ScanAddedEmpty` is `NotificationChannels` | `[]NotificationChannel` for all of them; `NotificationSettings` decodes DefectDojo's single-channel `scan_added_empty` itself. The patch's events are `*[]NotificationChannel`, its `ScanAddedEmpty` a `*NotificationChannel`. |

## Named string types

Fields with fixed values have named string types, and their constants are typed:

| Type | Fields |
| --- | --- |
| `AnnouncementStyle` | `Announcement.Style`, `AnnouncementPatch.Style`, `ListAnnouncementsOptions.Style` |
| `BusinessCriticality` | `Product.BusinessCriticality`, `ProductPatch.BusinessCriticality` |
| `CICDInfrastructureType` | `CICDInfrastructure.InfrastructureType`, the list and lookup options |
| `DeduplicationExecutionMode` (new) | `UserContactInfo.DeduplicationExecutionMode` |
| `EngagementStatus`, `EngagementType` | `Engagement.Status` and `EngagementType`, `EngagementPatch`, `ListEngagementsOptions.Status` |
| `JiraIssueType` (new) | `JiraInstance.DefaultIssueType`, `JiraInstancePatch.DefaultIssueType` |
| `NotificationChannel` | the notification events |
| `NotificationWebhookStatus` | `NotificationWebhook.Status`, `ListNotificationWebhooksOptions.Status` |
| `RegulationCategory` | `Regulation.Category`, `RegulationPatch.Category` |
| `Severity` (new) | `SystemSettings.JiraMinimumSeverity`, `SystemSettingsPatch.JiraMinimumSeverity` |

Constants and untyped literals still work (`Status: defectdojo.EngagementStatusCompleted`,
`Status: "Completed"`). A `string` variable needs a conversion: `defectdojo.EngagementStatus(s)`. In
`…Patch` fields, `new(...)` and `Value(...)` infer `string` from a literal, so pass a typed constant or a
conversion: `Status: new(defectdojo.EngagementStatusCompleted)`,
`BusinessCriticality: defectdojo.Value(defectdojo.BusinessCriticalityHigh)`. A value DefectDojo adds in a
later release works before it has a constant.

## Structs

Every exported struct, except the value types `Date`, `Nullable[T]` and `NotificationScope`, has an
unexported field. Literals must name their fields (`defectdojo.ProductType{Name: "Web"}`), and structs
can't be compared with `==` or used as map keys: compare them with `reflect.DeepEqual`, or compare their
fields. In return, a field added in a later v1 release can't break such code. `%+v` output now starts with
`_:[]`.

`JiraProject.ArchiveBatch` is left out of the JSON when it is empty, like the other `ArchiveBatch` fields.
