package rpc

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

const (
	maxPayloadBytes = 1 << 20
	maxTextRunes    = 512
)

var (
	idFormat        = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
	operationFormat = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{0,63}$`)
	assetTypeFormat = regexp.MustCompile(`^[A-Za-z0-9._-]{0,64}$`)
	keyFormat       = regexp.MustCompile(`^[A-Za-z0-9._:/-]{0,128}$`)

	// idempotencyKeyFormat leaves room for the sweep to prefix a step and
	// suffix an instant and still fit the 128 symbols of IDM-02.
	idempotencyKeyFormat = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
)

func invalid(field, rule string) error {
	return status.Error(codes.InvalidArgument, field+" "+rule)
}

func requireID(field, raw string) error {
	if !idFormat.MatchString(raw) {
		return invalid(field, "must have 1 to 128 characters of [A-Za-z0-9._:-]")
	}
	return nil
}

func optionalID(field, raw string) error {
	if raw == "" {
		return nil
	}
	return requireID(field, raw)
}

func requireIdempotencyKey(raw string) error {
	if !idempotencyKeyFormat.MatchString(raw) {
		return invalid("idempotency_key", "must have 1 to 100 characters of [A-Za-z0-9._-]")
	}
	return nil
}

func requireOperation(field, raw string, optional bool) error {
	if optional && raw == "" {
		return nil
	}
	if !operationFormat.MatchString(raw) {
		return invalid(field, "must be an operation name of 1 to 64 letters and digits")
	}
	return nil
}

func requireAssetType(raw string) error {
	if !assetTypeFormat.MatchString(raw) {
		return invalid("asset_type", "must have up to 64 characters of [A-Za-z0-9._-]")
	}
	return nil
}

// requireText refuses a NUL as well: the state goes to a jsonb snapshot, which
// rejects \u0000 with 22P05, an internal error instead of the caller's mistake.
func requireText(field, raw string, limit int) error {
	if !utf8.ValidString(raw) || utf8.RuneCountInString(raw) > limit || strings.ContainsRune(raw, 0) {
		return invalid(field, "must be valid UTF-8 without NUL within the length limit")
	}
	return nil
}

func requireNonEmptyText(field, raw string) error {
	if raw == "" {
		return invalid(field, "must not be empty")
	}
	return requireText(field, raw, maxTextRunes)
}

// requireDate takes the ISO 8601 calendar date the OpenAPI declares (format: date).
func requireDate(field, raw string) error {
	if _, err := time.Parse(time.DateOnly, raw); err != nil {
		return invalid(field, "must be an ISO 8601 date (YYYY-MM-DD)")
	}
	return nil
}

func requireVersion(field string, version int32) error {
	if version < 0 {
		return invalid(field, "must not be negative")
	}
	return nil
}

func rejectionOf(r *kernel.Rejection) *servicev1.Rejection {
	return &servicev1.Rejection{Code: string(r.Code()), Message: r.Message()}
}

var environmentsFromTheWire = map[servicev1.Environment]domain.Environment{
	servicev1.Environment_ENVIRONMENT_DEVELOPMENT:  domain.EnvironmentDevelopment,
	servicev1.Environment_ENVIRONMENT_INTEGRATION:  domain.EnvironmentIntegration,
	servicev1.Environment_ENVIRONMENT_HOMOLOGATION: domain.EnvironmentHomologation,
	servicev1.Environment_ENVIRONMENT_PRODUCTION:   domain.EnvironmentProduction,
}

var causesFromTheWire = map[servicev1.SuspensionCause]domain.SuspensionCause{
	servicev1.SuspensionCause_SUSPENSION_CAUSE_PARTNER:         domain.CausePartner,
	servicev1.SuspensionCause_SUSPENSION_CAUSE_SCHEMA_MISMATCH: domain.CauseSchemaMismatch,
}

var resolutionsFromTheWire = map[servicev1.Resolution]domain.Resolution{
	servicev1.Resolution_RESOLUTION_CONFIRMED:     domain.ResolutionConfirmed,
	servicev1.Resolution_RESOLUTION_FINAL_FAILURE: domain.ResolutionFinalFailure,
	servicev1.Resolution_RESOLUTION_NOT_RECEIVED:  domain.ResolutionNotReceived,
}

var protocolsFromTheWire = map[servicev1.Protocol]domain.Protocol{
	servicev1.Protocol_PROTOCOL_SYNC_HTTP:      domain.ProtocolSyncHTTP,
	servicev1.Protocol_PROTOCOL_ASYNC_CALLBACK: domain.ProtocolAsyncCallback,
	servicev1.Protocol_PROTOCOL_FILE_BATCH:     domain.ProtocolFileBatch,
	servicev1.Protocol_PROTOCOL_POLLING:        domain.ProtocolPolling,
}

var idempotencySupportFromTheWire = map[servicev1.IdempotencySupport]domain.IdempotencySupport{
	servicev1.IdempotencySupport_IDEMPOTENCY_SUPPORT_NATIVE:     domain.IdempotencyNative,
	servicev1.IdempotencySupport_IDEMPOTENCY_SUPPORT_KEY_HEADER: domain.IdempotencyKeyHeader,
	servicev1.IdempotencySupport_IDEMPOTENCY_SUPPORT_NONE:       domain.IdempotencyNone,
}

var coveragesFromTheWire = map[servicev1.EncumbranceCoverage]domain.EncumbranceCoverage{
	servicev1.EncumbranceCoverage_ENCUMBRANCE_COVERAGE_COMPLETE:    domain.CoverageComplete,
	servicev1.EncumbranceCoverage_ENCUMBRANCE_COVERAGE_PARTIAL:     domain.CoveragePartial,
	servicev1.EncumbranceCoverage_ENCUMBRANCE_COVERAGE_UNAVAILABLE: domain.CoverageUnavailable,
}

var efficaciesFromTheWire = map[servicev1.Efficacy]domain.Efficacy{
	servicev1.Efficacy_EFFICACY_ON_SUBMIT:      domain.EfficacyOnSubmit,
	servicev1.Efficacy_EFFICACY_BY_OBSERVATION: domain.EfficacyByObservation,
}

var statusesFromTheWire = map[servicev1.ExternalCommandStatus]domain.CommandStatus{
	servicev1.ExternalCommandStatus_EXTERNAL_COMMAND_STATUS_CREATED:           domain.StatusCreated,
	servicev1.ExternalCommandStatus_EXTERNAL_COMMAND_STATUS_ACKNOWLEDGED:      domain.StatusAcknowledged,
	servicev1.ExternalCommandStatus_EXTERNAL_COMMAND_STATUS_CONFIRMED:         domain.StatusConfirmed,
	servicev1.ExternalCommandStatus_EXTERNAL_COMMAND_STATUS_RETRYABLE_FAILURE: domain.StatusRetryableFailure,
	servicev1.ExternalCommandStatus_EXTERNAL_COMMAND_STATUS_FINAL_FAILURE:     domain.StatusFinalFailure,
	servicev1.ExternalCommandStatus_EXTERNAL_COMMAND_STATUS_UNKNOWN_OUTCOME:   domain.StatusUnknownOutcome,
	servicev1.ExternalCommandStatus_EXTERNAL_COMMAND_STATUS_CANCELLED:         domain.StatusCancelled,
}

var failureCategoriesToTheWire = map[domain.FailureCategory]servicev1.FailureCategory{
	domain.CategoryFinalValidation:    servicev1.FailureCategory_FAILURE_CATEGORY_FINAL_VALIDATION,
	domain.CategoryFinalBusiness:      servicev1.FailureCategory_FAILURE_CATEGORY_FINAL_BUSINESS,
	domain.CategoryFinalAuthorization: servicev1.FailureCategory_FAILURE_CATEGORY_FINAL_AUTHORIZATION,
	domain.CategoryUnknownSent:        servicev1.FailureCategory_FAILURE_CATEGORY_UNKNOWN_SENT,
	domain.CategoryUnknownAmbiguous:   servicev1.FailureCategory_FAILURE_CATEGORY_UNKNOWN_AMBIGUOUS,
	domain.CategorySchemaMismatch:     servicev1.FailureCategory_FAILURE_CATEGORY_SCHEMA_MISMATCH,
}

var authorizationStatusesToTheWire = map[domain.AuthorizationStatus]servicev1.AuthorizationStatus{
	domain.AuthorizationGranted: servicev1.AuthorizationStatus_AUTHORIZATION_STATUS_GRANTED,
	domain.AuthorizationRevoked: servicev1.AuthorizationStatus_AUTHORIZATION_STATUS_REVOKED,
}

// toTheWire inverts a map from the wire; a value the map does not know
// travels as the zero, UNSPECIFIED, never as a guess.
func toTheWire[W comparable, D comparable](from map[W]D) map[D]W {
	out := make(map[D]W, len(from))
	for wire, value := range from {
		out[value] = wire
	}
	return out
}

var (
	environmentsToTheWire       = toTheWire(environmentsFromTheWire)
	causesToTheWire             = toTheWire(causesFromTheWire)
	protocolsToTheWire          = toTheWire(protocolsFromTheWire)
	idempotencySupportToTheWire = toTheWire(idempotencySupportFromTheWire)
	coveragesToTheWire          = toTheWire(coveragesFromTheWire)
	efficaciesToTheWire         = toTheWire(efficaciesFromTheWire)
	statusesToTheWire           = toTheWire(statusesFromTheWire)
)

func enumFromTheWire[W comparable, D any](field string, from map[W]D, raw W) (D, error) {
	value, known := from[raw]
	if !known {
		var none D
		return none, invalid(field, "must be one of the declared values")
	}
	return value, nil
}

func onTheWire[S any, W any](found []S, convert func(S) *W) []*W {
	out := make([]*W, 0, len(found))
	for _, s := range found {
		out = append(out, convert(s))
	}
	return out
}

func connectorOnTheWire(s domain.ConnectorSnapshot) *servicev1.Connector {
	capabilities := make([]*servicev1.EnabledCapability, 0, len(s.Capabilities))
	for _, c := range s.Capabilities {
		capabilities = append(capabilities, &servicev1.EnabledCapability{
			CapabilityId: string(c.CapabilityID), Operation: c.Operation, AssetType: c.AssetType,
			StateQuerySupported: c.StateQuerySupported, Version: int32(c.Version), At: int64(c.At),
		})
	}
	suspensions := make([]*servicev1.OperationSuspension, 0, len(s.Suspensions))
	for _, x := range s.Suspensions {
		suspensions = append(suspensions, &servicev1.OperationSuspension{
			Operation: x.Operation, AssetType: x.AssetType, Cause: causesToTheWire[x.Cause],
			CapabilityId: string(x.CapabilityID), Version: int32(x.Version), Since: int64(x.Since),
		})
	}
	return &servicev1.Connector{
		ConnectorKey: string(s.Key), PartnerId: string(s.PartnerID), Environment: environmentsToTheWire[s.Environment],
		CredentialRef: s.CredentialRef, IntegrationVersion: int32(s.IntegrationVersion),
		Capabilities: capabilities, Suspensions: suspensions, RegisteredAt: int64(s.RegisteredAt),
	}
}

func operationOnTheWire(o domain.OperationCapability) *servicev1.OperationCapability {
	return &servicev1.OperationCapability{
		Operation: o.Operation, Supported: o.Supported, Protocol: protocolsToTheWire[o.Protocol], StateQuerySupported: o.StateQuerySupported,
		IdempotencySupport: idempotencySupportToTheWire[o.IdempotencySupport], PartialResultSupported: o.PartialResultSupported,
		AssetTypes: append([]string(nil), o.AssetTypes...), MaxItemsPerRequest: int32(o.MaxItemsPerRequest),
		MaxRequestsPerMinute: int32(o.MaxRequestsPerMinute), MaxPayloadBytes: int32(o.MaxPayloadBytes),
		BusinessCalendar: o.BusinessCalendar, CutoffTime: o.CutoffTime, ProcessingWindow: o.ProcessingWindow,
		EncumbranceCoverage: coveragesToTheWire[o.EncumbranceCoverage], PositionFreshnessSla: int32(o.PositionFreshnessSLA),
		Efficacy: efficaciesToTheWire[o.Efficacy], Quiescence: o.Quiescence, AuthoritativeAbsence: o.AuthoritativeAbsence,
		MaxQueries: int32(o.MaxQueries), KeyAlphabet: o.KeyAlphabet, KeyMaxLength: int32(o.KeyMaxLength),
		QueryInterval: int32(o.QueryInterval), AmbiguityDeadline: int32(o.AmbiguityDeadline),
	}
}

func profileOnTheWire(s domain.CapabilityProfileSnapshot) *servicev1.CapabilityProfile {
	return &servicev1.CapabilityProfile{
		ProfileKey: string(s.Key), ConnectorId: string(s.ConnectorKey), Version: int32(s.Version), EffectiveFrom: int64(s.EffectiveFrom),
		Operations: onTheWire(s.Operations, operationOnTheWire),
	}
}

func authorizationOnTheWire(s domain.QueryAuthorizationSnapshot) *servicev1.QueryAuthorization {
	return &servicev1.QueryAuthorization{
		ConsentId: string(s.ConsentID), OrganizationId: string(s.OrganizationID), ConsentVersion: int32(s.ConsentVersion),
		Status: authorizationStatusesToTheWire[s.Status], AssetTypes: append([]string(nil), s.AssetTypes...),
		PeriodFromDate: s.PeriodFromDate, PeriodUntilDate: s.PeriodUntilDate, GrantedAt: int64(s.GrantedAt), RevokedAt: int64(s.RevokedAt),
	}
}

// commandOnTheWire leaves the payload out: it is CONFIDENTIAL, and its hash is
// what a caller compares.
func commandOnTheWire(s domain.ExternalCommandSnapshot) *servicev1.ExternalCommand {
	p := s.Profile
	return &servicev1.ExternalCommand{
		IdempotencyKey: string(s.Key), TenantId: string(s.TenantID), ConnectorId: string(s.ConnectorKey), Operation: s.Operation,
		AssetType: s.AssetType, BusinessKey: s.BusinessKey, PayloadVersion: int32(s.PayloadVersion), SeriesKey: s.SeriesKey,
		PayloadHash: s.PayloadHash,
		Profile: &servicev1.CommandProfile{
			ProfileVersion: int32(p.ProfileVersion), StateQuerySupported: p.StateQuerySupported,
			IdempotencySupport: idempotencySupportToTheWire[p.IdempotencySupport], PartialResultSupported: p.PartialResultSupported,
			Efficacy: efficaciesToTheWire[p.Efficacy], Quiescence: p.Quiescence, AuthoritativeAbsence: p.AuthoritativeAbsence,
			MaxQueries: int32(p.MaxQueries), KeyAlphabet: p.KeyAlphabet, KeyMaxLength: int32(p.KeyMaxLength),
			QueryInterval: int32(p.QueryInterval), AmbiguityDeadline: int32(p.AmbiguityDeadline),
		},
		Status: statusesToTheWire[s.Status], FailureCategory: failureCategoriesToTheWire[s.FailureCategory],
		CreatedAt: int64(s.CreatedAt), Submissions: int32(s.Submissions), LastSubmittedAt: int64(s.LastSubmittedAt),
		WaitingSince: int64(s.WaitingSince), NextQueryAt: int64(s.NextQueryAt), ResendAfter: int64(s.ResendAfter),
		ReconciliationRequired: s.ReconciliationRequired, AmbiguityExpiredAt: int64(s.AmbiguityExpiredAt),
		ConfirmedAt: int64(s.ConfirmedAt), ExternalReferenceId: string(s.ExternalReferenceID),
	}
}
