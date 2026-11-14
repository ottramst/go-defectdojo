package defectdojo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

// SystemSettingsService handles communication with the system settings
// methods of the DefectDojo API. The system settings are a single object that
// configures the whole instance: deduplication, Jira, notification channels,
// SLA notifications, grading, password rules and more. Only superusers can
// read or change them.
//
// The settings can't be created or deleted, and DefectDojo has no GET by ID:
// [SystemSettingsService.Get] lists them. There is no Update either: no field
// is required, so a PUT keeps the fields it leaves out, as
// [SystemSettingsService.Patch] does.
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type SystemSettingsService service

// SystemSettings are DefectDojo's system settings.
//
// DefectDojo returns the secrets SlackToken, MSTeamsURL and JiraWebhookSecret
// in plain text.
type SystemSettings struct {
	ID int `json:"id"`

	// EnableDeduplication marks new findings that match an existing finding
	// as duplicates. With DeleteDuplicates, DefectDojo deletes the oldest
	// duplicates of a finding beyond MaxDupes. MaxDupes is nil for no limit;
	// 0 deletes every duplicate.
	EnableDeduplication bool `json:"enable_deduplication"`
	DeleteDuplicates    bool `json:"delete_duplicates"`
	MaxDupes            *int `json:"max_dupes"`

	// EmailFrom is the sender of notification mails (at most 200
	// characters).
	EmailFrom string `json:"email_from"`

	// EnableJira turns the Jira integration on. EnableJiraWebhook accepts
	// Jira's webhook calls, which need JiraWebhookSecret in their URL unless
	// DisableJiraWebhookSecret is set. DefectDojo generates the secret (at
	// most 64 characters) when it is installed.
	EnableJira               bool   `json:"enable_jira"`
	EnableJiraWebhook        bool   `json:"enable_jira_web_hook"`
	DisableJiraWebhookSecret bool   `json:"disable_jira_webhook_secret"`
	JiraWebhookSecret        string `json:"jira_webhook_secret"`

	// JiraMinimumSeverity is the lowest severity of findings that are pushed
	// to Jira: "Critical", "High", "Medium", "Low", "Info", or "" for all.
	JiraMinimumSeverity string `json:"jira_minimum_severity"`

	// JiraLabels are the labels of new Jira issues, separated by spaces (at
	// most 200 characters).
	JiraLabels                    string `json:"jira_labels"`
	AddVulnerabilityIDToJiraLabel bool   `json:"add_vulnerability_id_to_jira_label"`

	EnableGitHub bool `json:"enable_github"`

	// The notification channels. A channel must be enabled here for the
	// notification settings to use it. SlackChannel, SlackToken and
	// SlackUsername are at most 100 characters, MSTeamsURL (the incoming
	// webhook URL, which carries its token) at most 400, and
	// MailNotificationsTo at most 200. WebhooksNotificationsTimeout is in
	// seconds.
	EnableSlackNotifications     bool   `json:"enable_slack_notifications"`
	SlackChannel                 string `json:"slack_channel"`
	SlackToken                   string `json:"slack_token"`
	SlackUsername                string `json:"slack_username"`
	EnableMSTeamsNotifications   bool   `json:"enable_msteams_notifications"`
	MSTeamsURL                   string `json:"msteams_url"`
	EnableMailNotifications      bool   `json:"enable_mail_notifications"`
	MailNotificationsTo          string `json:"mail_notifications_to"`
	EnableWebhooksNotifications  bool   `json:"enable_webhooks_notifications"`
	WebhooksNotificationsTimeout int    `json:"webhooks_notifications_timeout"`

	// EnforceVerifiedStatus makes grading, Jira, metrics and reports consider
	// only verified findings, overriding the three scoped settings.
	EnforceVerifiedStatus               bool `json:"enforce_verified_status"`
	EnforceVerifiedStatusJira           bool `json:"enforce_verified_status_jira"`
	EnforceVerifiedStatusProductGrading bool `json:"enforce_verified_status_product_grading"`
	EnforceVerifiedStatusMetrics        bool `json:"enforce_verified_status_metrics"`
	FalsePositiveHistory                bool `json:"false_positive_history"`
	RetroactiveFalsePositiveHistory     bool `json:"retroactive_false_positive_history"`

	// URLPrefix is the path DefectDojo is served under (at most 300
	// characters), and TeamName the name of the team (at most 100).
	URLPrefix string `json:"url_prefix"`
	TeamName  string `json:"team_name"`

	// The product grades: the percentage score a product needs for an A, B,
	// C or D, and the score at or below which it gets an F.
	EnableProductGrade bool `json:"enable_product_grade"`
	ProductGradeA      int  `json:"product_grade_a"`
	ProductGradeB      int  `json:"product_grade_b"`
	ProductGradeC      int  `json:"product_grade_c"`
	ProductGradeD      int  `json:"product_grade_d"`
	ProductGradeF      int  `json:"product_grade_f"`

	EnableProductTagInheritance bool `json:"enable_product_tag_inheritance"`
	EnableBenchmark             bool `json:"enable_benchmark"`
	EnableSimilarFindings       bool `json:"enable_similar_findings"`

	// EngagementAutoClose closes engagements EngagementAutoCloseDays days
	// after their end date.
	EngagementAutoClose     bool `json:"engagement_auto_close"`
	EngagementAutoCloseDays int  `json:"engagement_auto_close_days"`

	// EnableFindingSLA turns finding SLAs on. While it is off, DefectDojo
	// never finishes recalculating SLA dates, so SLA configurations whose
	// days changed stay locked; see [IsSLARecalculating].
	EnableFindingSLA                  bool `json:"enable_finding_sla"`
	EnableNotifySLAActive             bool `json:"enable_notify_sla_active"`
	EnableNotifySLAActiveVerified     bool `json:"enable_notify_sla_active_verified"`
	EnableNotifySLAJiraOnly           bool `json:"enable_notify_sla_jira_only"`
	EnableNotifySLAExponentialBackoff bool `json:"enable_notify_sla_exponential_backoff"`

	// AllowAnonymousSurveyResponse lets anyone with a survey's link answer
	// it. Its JSON name misspells "response", as the API does.
	AllowAnonymousSurveyResponse bool `json:"allow_anonymous_survey_repsonse"` //nolint:misspell // DefectDojo's spelling.

	// The disclaimers: DisclaimerNotifications and DisclaimerNotes are at most
	// 3000 characters, DisclaimerReports at most 5000.
	DisclaimerNotifications string `json:"disclaimer_notifications"`
	DisclaimerReports       string `json:"disclaimer_reports"`
	DisclaimerReportsForced bool   `json:"disclaimer_reports_forced"`
	DisclaimerNotes         string `json:"disclaimer_notes"`

	// RiskAcceptanceFormDefaultDays is the default expiry of risk acceptances
	// in days, and RiskAcceptanceNotifyBeforeExpiration how many days before
	// it DefectDojo notifies; nil turns either off.
	RiskAcceptanceFormDefaultDays        *int `json:"risk_acceptance_form_default_days"`
	RiskAcceptanceNotifyBeforeExpiration *int `json:"risk_acceptance_notify_before_expiration"`

	EnableQuestionnaires         bool `json:"enable_questionnaires"`
	EnableChecklists             bool `json:"enable_checklists"`
	EnableEndpointMetadataImport bool `json:"enable_endpoint_metadata_import"`
	EnableUserProfileEditable    bool `json:"enable_user_profile_editable"`
	EnableProductTrackingFiles   bool `json:"enable_product_tracking_files"`
	EnableFindingGroups          bool `json:"enable_finding_groups"`
	EnableUITableBasedSearching  bool `json:"enable_ui_table_based_searching"`
	EnableCalendar               bool `json:"enable_calendar"`
	EnableCVSS3Display           bool `json:"enable_cvss3_display"`
	EnableCVSS4Display           bool `json:"enable_cvss4_display"`

	// The password rules of users' passwords. The lengths are 9 to 48.
	MinimumPasswordLength      int  `json:"minimum_password_length"`
	MaximumPasswordLength      int  `json:"maximum_password_length"`
	NumberCharacterRequired    bool `json:"number_character_required"`
	SpecialCharacterRequired   bool `json:"special_character_required"`
	LowercaseCharacterRequired bool `json:"lowercase_character_required"`
	UppercaseCharacterRequired bool `json:"uppercase_character_required"`
	NonCommonPasswordRequired  bool `json:"non_common_password_required"`

	APIExposeErrorDetails bool `json:"api_expose_error_details"`
	FilterStringMatching  bool `json:"filter_string_matching"`

	// ReadOnlyMode rejects every change through the UI and the API, except
	// to the system settings, with a 423 ([ErrLocked]), and pauses scheduled
	// background jobs. It is new in DefectDojo 3.5.100 and nil on older servers.
	ReadOnlyMode *bool `json:"read_only_mode"`

	// InstallID identifies the DefectDojo installation. It is read-only.
	InstallID string `json:"install_id"`
}

// SystemSettingsPatch is the body of [SystemSettingsService.Patch]. Only the
// fields that are set are changed. See [SystemSettings] for the fields.
//
// Text fields that aren't nullable are cleared with "", nullable ones with
// [Null]. JiraWebhookSecret can't be "".
type SystemSettingsPatch struct {
	EnableDeduplication *bool         `json:"enable_deduplication,omitempty"`
	DeleteDuplicates    *bool         `json:"delete_duplicates,omitempty"`
	MaxDupes            Nullable[int] `json:"max_dupes,omitzero"`
	EmailFrom           *string       `json:"email_from,omitempty"`

	EnableJira                    *bool            `json:"enable_jira,omitempty"`
	EnableJiraWebhook             *bool            `json:"enable_jira_web_hook,omitempty"`
	DisableJiraWebhookSecret      *bool            `json:"disable_jira_webhook_secret,omitempty"`
	JiraWebhookSecret             Nullable[string] `json:"jira_webhook_secret,omitzero"`
	JiraMinimumSeverity           Nullable[string] `json:"jira_minimum_severity,omitzero"`
	JiraLabels                    Nullable[string] `json:"jira_labels,omitzero"`
	AddVulnerabilityIDToJiraLabel *bool            `json:"add_vulnerability_id_to_jira_label,omitempty"`

	EnableGitHub *bool `json:"enable_github,omitempty"`

	EnableSlackNotifications     *bool   `json:"enable_slack_notifications,omitempty"`
	SlackChannel                 *string `json:"slack_channel,omitempty"`
	SlackToken                   *string `json:"slack_token,omitempty"`
	SlackUsername                *string `json:"slack_username,omitempty"`
	EnableMSTeamsNotifications   *bool   `json:"enable_msteams_notifications,omitempty"`
	MSTeamsURL                   *string `json:"msteams_url,omitempty"`
	EnableMailNotifications      *bool   `json:"enable_mail_notifications,omitempty"`
	MailNotificationsTo          *string `json:"mail_notifications_to,omitempty"`
	EnableWebhooksNotifications  *bool   `json:"enable_webhooks_notifications,omitempty"`
	WebhooksNotificationsTimeout *int    `json:"webhooks_notifications_timeout,omitempty"`

	EnforceVerifiedStatus               *bool `json:"enforce_verified_status,omitempty"`
	EnforceVerifiedStatusJira           *bool `json:"enforce_verified_status_jira,omitempty"`
	EnforceVerifiedStatusProductGrading *bool `json:"enforce_verified_status_product_grading,omitempty"`
	EnforceVerifiedStatusMetrics        *bool `json:"enforce_verified_status_metrics,omitempty"`
	FalsePositiveHistory                *bool `json:"false_positive_history,omitempty"`
	RetroactiveFalsePositiveHistory     *bool `json:"retroactive_false_positive_history,omitempty"`

	URLPrefix *string `json:"url_prefix,omitempty"`
	TeamName  *string `json:"team_name,omitempty"`

	EnableProductGrade *bool `json:"enable_product_grade,omitempty"`
	ProductGradeA      *int  `json:"product_grade_a,omitempty"`
	ProductGradeB      *int  `json:"product_grade_b,omitempty"`
	ProductGradeC      *int  `json:"product_grade_c,omitempty"`
	ProductGradeD      *int  `json:"product_grade_d,omitempty"`
	ProductGradeF      *int  `json:"product_grade_f,omitempty"`

	EnableProductTagInheritance *bool `json:"enable_product_tag_inheritance,omitempty"`
	EnableBenchmark             *bool `json:"enable_benchmark,omitempty"`
	EnableSimilarFindings       *bool `json:"enable_similar_findings,omitempty"`

	EngagementAutoClose     *bool `json:"engagement_auto_close,omitempty"`
	EngagementAutoCloseDays *int  `json:"engagement_auto_close_days,omitempty"`

	EnableFindingSLA                  *bool `json:"enable_finding_sla,omitempty"`
	EnableNotifySLAActive             *bool `json:"enable_notify_sla_active,omitempty"`
	EnableNotifySLAActiveVerified     *bool `json:"enable_notify_sla_active_verified,omitempty"`
	EnableNotifySLAJiraOnly           *bool `json:"enable_notify_sla_jira_only,omitempty"`
	EnableNotifySLAExponentialBackoff *bool `json:"enable_notify_sla_exponential_backoff,omitempty"`

	AllowAnonymousSurveyResponse *bool `json:"allow_anonymous_survey_repsonse,omitempty"` //nolint:misspell // DefectDojo's spelling.

	DisclaimerNotifications *string `json:"disclaimer_notifications,omitempty"`
	DisclaimerReports       *string `json:"disclaimer_reports,omitempty"`
	DisclaimerReportsForced *bool   `json:"disclaimer_reports_forced,omitempty"`
	DisclaimerNotes         *string `json:"disclaimer_notes,omitempty"`

	RiskAcceptanceFormDefaultDays        Nullable[int] `json:"risk_acceptance_form_default_days,omitzero"`
	RiskAcceptanceNotifyBeforeExpiration Nullable[int] `json:"risk_acceptance_notify_before_expiration,omitzero"`

	EnableQuestionnaires         *bool `json:"enable_questionnaires,omitempty"`
	EnableChecklists             *bool `json:"enable_checklists,omitempty"`
	EnableEndpointMetadataImport *bool `json:"enable_endpoint_metadata_import,omitempty"`
	EnableUserProfileEditable    *bool `json:"enable_user_profile_editable,omitempty"`
	EnableProductTrackingFiles   *bool `json:"enable_product_tracking_files,omitempty"`
	EnableFindingGroups          *bool `json:"enable_finding_groups,omitempty"`
	EnableUITableBasedSearching  *bool `json:"enable_ui_table_based_searching,omitempty"`
	EnableCalendar               *bool `json:"enable_calendar,omitempty"`
	EnableCVSS3Display           *bool `json:"enable_cvss3_display,omitempty"`
	EnableCVSS4Display           *bool `json:"enable_cvss4_display,omitempty"`

	// MinimumPasswordLength and MaximumPasswordLength must be 9 to 48, and
	// when both are set, the minimum must not exceed the maximum: DefectDojo
	// doesn't check that, so the client does. A patch of only one of them is
	// not compared with the other's current value.
	MinimumPasswordLength      *int  `json:"minimum_password_length,omitempty"`
	MaximumPasswordLength      *int  `json:"maximum_password_length,omitempty"`
	NumberCharacterRequired    *bool `json:"number_character_required,omitempty"`
	SpecialCharacterRequired   *bool `json:"special_character_required,omitempty"`
	LowercaseCharacterRequired *bool `json:"lowercase_character_required,omitempty"`
	UppercaseCharacterRequired *bool `json:"uppercase_character_required,omitempty"`
	NonCommonPasswordRequired  *bool `json:"non_common_password_required,omitempty"`

	APIExposeErrorDetails *bool `json:"api_expose_error_details,omitempty"`
	FilterStringMatching  *bool `json:"filter_string_matching,omitempty"`

	// ReadOnlyMode is new in DefectDojo 3.5.100; older servers ignore it and
	// answer 200 without changing anything.
	ReadOnlyMode *bool `json:"read_only_mode,omitempty"`
}

// checkPasswordLengths returns an error if a password length that is set is
// outside 9 to 48, or if both are set and the minimum exceeds the maximum.
func checkPasswordLengths(minimum, maximum *int) error {
	for _, l := range []struct {
		name   string
		length *int
	}{{"MinimumPasswordLength", minimum}, {"MaximumPasswordLength", maximum}} {
		if l.length != nil && (*l.length < 9 || *l.length > 48) {
			return fmt.Errorf("defectdojo: system settings %s must be 9 to 48, got %d", l.name, *l.length)
		}
	}
	if minimum != nil && maximum != nil && *minimum > *maximum {
		return fmt.Errorf("defectdojo: system settings MinimumPasswordLength %d exceeds MaximumPasswordLength %d", *minimum, *maximum)
	}
	return nil
}

// Get returns the system settings. DefectDojo lists them as a single object;
// Get returns an error if the list doesn't hold exactly one.
//
// DefectDojo API: GET /api/v2/system_settings/
func (s *SystemSettingsService) Get(ctx context.Context, opts ...RequestOption) (*SystemSettings, *Response, error) {
	var page listPage[*SystemSettings]
	resp, err := s.client.do(ctx, http.MethodGet, "/api/v2/system_settings/", nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	if page.Count != 1 || len(page.Results) != 1 {
		return nil, resp, fmt.Errorf("defectdojo: expected one system settings object, got %d", page.Count)
	}
	return page.Results[0], resp, nil
}

// Patch changes the system settings that are set in patch. The ID is the one
// [SystemSettingsService.Get] returns. Password lengths outside 9 to 48, and a
// minimum above the maximum when both are set, are rejected before sending.
//
// DefectDojo API: PATCH /api/v2/system_settings/{id}/
func (s *SystemSettingsService) Patch(ctx context.Context, id int, patch *SystemSettingsPatch, opts ...RequestOption) (*SystemSettings, *Response, error) {
	if err := requirePositive("system settings ID", id); err != nil {
		return nil, nil, err
	}
	if patch == nil {
		return nil, nil, errors.New("defectdojo: system settings patch must not be nil")
	}
	if err := checkPasswordLengths(patch.MinimumPasswordLength, patch.MaximumPasswordLength); err != nil {
		return nil, nil, err
	}
	updated := new(SystemSettings)
	resp, err := s.client.do(ctx, http.MethodPatch, "/api/v2/system_settings/"+strconv.Itoa(id)+"/", patch, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}
