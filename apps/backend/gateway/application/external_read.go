package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

var (
	// ErrNotAReadOperation is a read named by an operation outside the reads
	// of the ports.
	ErrNotAReadOperation = errors.New("application: the operation is not a read of the ports")

	// ErrReadingNotNormalized is a reading the partner gave outside its
	// contract: the read stays recorded, and its normalization detects the
	// mismatch that suspends the operation.
	ErrReadingNotNormalized = errors.New("application: the reading of the partner is not normalized")
)

// ExternalReading is the answer of a read with its provenance (decision 20).
// Outcome is how the normalizer read it, kept so a replay answers as the first
// call did.
type ExternalReading struct {
	InteractionID     domain.InteractionID
	AsOf              kernel.Instant
	ProviderID        domain.PartnerID
	ExternalReference string
	Reading           string
	ReadingHash       string
	Outcome           domain.InteractionStatus
}

var readingCodec = usecase.OutcomeCodec[ExternalReading]{
	Encode: func(e *usecase.Encoder, r ExternalReading) {
		e.String(string(r.InteractionID))
		e.Int(int64(r.AsOf))
		e.String(string(r.ProviderID))
		e.String(r.ExternalReference)
		e.String(r.Reading)
		e.String(r.ReadingHash)
		e.String(string(r.Outcome))
	},
	Decode: func(d *usecase.Decoder) ExternalReading {
		var r ExternalReading
		r.InteractionID = domain.InteractionID(d.String())
		r.AsOf = kernel.Instant(d.Int())
		r.ProviderID = domain.PartnerID(d.String())
		r.ExternalReference = d.String()
		r.Reading = d.String()
		r.ReadingHash = d.String()
		r.Outcome = domain.InteractionStatus(d.String())
		return r
	},
}

// ReadExternal reads from a partner through a canonical port, in the order of
// the spec: the guard of the port, the call to the adapter with both raw sides
// kept, and only then the record of the external_read interaction under the
// caller's command key. A replay of the key calls the partner again and
// answers the reading recorded the first time.
func (s Service) ReadExternal(ctx context.Context, cmd ReadExternal) (usecase.Outcome[ExternalReading], error) {
	var none usecase.Outcome[ExternalReading]
	if !domain.IsReadOperation(cmd.Operation) {
		return none, usecase.NewFailure(usecase.Validation, false,
			fmt.Errorf("application: %s %s: %w", OperationReadExternal, cmd.Operation, ErrNotAReadOperation))
	}
	key, keyed := port.IdempotencyKeyFrom(ctx)
	if !keyed || key == "" {
		return none, port.ErrIdempotencyKeyAbsent
	}
	if err := s.Authorize(ctx, cmd); err != nil {
		return none, err
	}
	guarded, refusal, err := s.guardRead(ctx, cmd)
	switch {
	case err != nil:
		return none, fmt.Errorf("application: read external %s %s: %w", cmd.ConnectorKey, cmd.Operation, err)
	case refusal != nil:
		return usecase.Rejected[ExternalReading](refusal), nil
	}
	id, err := s.DerivedKeys.ExternalReadInteractionID(cmd.ConnectorKey, key)
	if err != nil {
		return none, fmt.Errorf("application: read external %s %s: %w", cmd.ConnectorKey, cmd.Operation, err)
	}
	read, err := s.readFromPartner(ctx, cmd, id)
	if err != nil {
		return none, fmt.Errorf("application: read external %s: %w", id, err)
	}
	outcome, err := usecase.Execute(ctx, s.executor(), command[ExternalReading]{
		Operation: OperationReadExternal,
		Object:    string(id),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationReadExternal).
			String(string(cmd.ConnectorKey)).String(cmd.Operation).String(string(cmd.OrganizationID)).String(cmd.AssetType).
			String(string(cmd.Parameters)),
		Codec: readingCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[ExternalReading], error) {
			recorded, err := recordIn(ctx, res, identity, read.record)
			if err != nil {
				return usecase.Outcome[ExternalReading]{}, fmt.Errorf("application: read external %s: %w", id, err)
			}
			if rejection, refused := recorded.Rejection(); refused {
				return usecase.Rejected[ExternalReading](rejection), nil
			}
			return answerOf(id, guarded, read.reading, identity.OccurredAt), nil
		},
	})
	if err != nil {
		return outcome, err
	}
	if _, refused := outcome.Rejection(); !refused && outcome.Response().Outcome != domain.InteractionNormalized {
		return none, usecase.NewFailure(usecase.Unexpected, false, fmt.Errorf("application: read external %s: %w", id, ErrReadingNotNormalized))
	}
	return outcome, nil
}

type readGuard struct {
	provider    domain.PartnerID
	declaration domain.OperationCapability
}

// guardRead is the guard of the port, before any call (fail-closed): an
// authorization of query for the reads of ReceivablesQueryPort, then the
// operation supported by the profile in force and enabled, unsuspended, for
// the connector.
func (s Service) guardRead(ctx context.Context, cmd ReadExternal) (readGuard, *kernel.Rejection, error) {
	if domain.RequiresQueryAuthorization(cmd.Operation) {
		authorizations, err := s.AuthorizationsByOrganization.LoadByOrganization(ctx, cmd.OrganizationID)
		if err != nil && !isNotFound(err) {
			return readGuard{}, nil, err
		}
		active := func(a domain.QueryAuthorizationSnapshot) bool { return a.Active(cmd.AssetType) }
		if !slices.ContainsFunc(authorizations, active) {
			return readGuard{}, domain.RejectQueryAuthorizationConsentNotActive, nil
		}
	}
	connector, _, err := s.ConnectorReader.Load(ctx, cmd.ConnectorKey)
	switch {
	case isNotFound(err):
		return readGuard{}, domain.RejectConnectorCapabilityNotEnabled, nil
	case err != nil:
		return readGuard{}, nil, err
	}
	profiles, err := s.ProfilesByConnector.LoadByConnector(ctx, cmd.ConnectorKey)
	if err != nil && !isNotFound(err) {
		return readGuard{}, nil, err
	}
	effective, inForce := domain.EffectiveProfile(profiles, s.Clock.Now())
	declaration, supported := effective.Supports(cmd.Operation, cmd.AssetType)
	if gate, _ := connector.Gate(cmd.Operation, cmd.AssetType); !inForce || !supported || gate != domain.GateAllowed {
		return readGuard{}, domain.RejectConnectorCapabilityNotEnabled, nil
	}
	return readGuard{provider: connector.PartnerID, declaration: declaration}, nil, nil
}

type partnerRead struct {
	record  RecordExternalInteraction
	reading ports.Reading
}

// readFromPartner fails closed before the call when the connector has no
// reader or normalizer, or the raw store is not there: an answer that could
// not be kept is not asked for.
func (s Service) readFromPartner(ctx context.Context, cmd ReadExternal, id domain.InteractionID) (partnerRead, error) {
	reader, err := s.partners().Reader(cmd.ConnectorKey, cmd.Operation)
	if err != nil {
		return partnerRead{}, dependency(err)
	}
	normalizer, err := s.partners().Normalizer(cmd.ConnectorKey, cmd.Operation)
	if err != nil {
		return partnerRead{}, dependency(err)
	}
	if s.Raw == nil {
		return partnerRead{}, dependency(ports.ErrRawStoreUnavailable)
	}
	exchange, err := reader.Read(ctx, cmd.Parameters)
	if err != nil {
		return partnerRead{}, dependency(err)
	}
	request, err := s.keep(ctx, cmd.ConnectorKey, id, ports.RawRequest, exchange.Request)
	if err != nil {
		return partnerRead{}, err
	}
	response, err := s.keep(ctx, cmd.ConnectorKey, id, ports.RawResponse, exchange.Response)
	if err != nil {
		return partnerRead{}, err
	}
	reading, err := normalizer.Normalize(domain.InteractionExternalRead, exchange)
	if err != nil {
		return partnerRead{}, usecase.NewFailure(usecase.Unexpected, false, err)
	}
	return partnerRead{
		record: RecordExternalInteraction{
			InteractionID: id, ConnectorKey: cmd.ConnectorKey, Operation: cmd.Operation, Direction: domain.DirectionOutbound,
			Source: domain.InteractionExternalRead, ContentType: exchange.Response.ContentType, Request: request, Response: response,
		},
		reading: reading,
	}, nil
}

// answerOf bounds what the command inbox keeps by the maxPayloadBytes of the
// operation (decision 20); a reading the partner gave without an instant is
// as of its record.
func answerOf(id domain.InteractionID, guarded readGuard, reading ports.Reading, recordedAt kernel.Instant) usecase.Outcome[ExternalReading] {
	answer := ExternalReading{
		InteractionID: id, AsOf: reading.AsOf, ProviderID: guarded.provider, ExternalReference: reading.ExternalRef, Outcome: reading.Outcome,
	}
	if answer.AsOf == 0 {
		answer.AsOf = recordedAt
	}
	if reading.Outcome != domain.InteractionNormalized {
		return usecase.Accepted(answer)
	}
	if len(reading.Canonical) > guarded.declaration.MaxPayloadBytes {
		return usecase.Rejected[ExternalReading](domain.RejectExternalInteractionPayloadTooLarge)
	}
	answer.Reading, answer.ReadingHash = string(reading.Canonical), reading.ReadingHash
	return usecase.Accepted(answer)
}
