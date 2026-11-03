package application_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit/partnersim"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

// countedReads counts the reads that reach the partner.
type countedReads struct {
	ports.PartnerAdapters
	mu sync.Mutex
	n  int
}

func (c *countedReads) Reader(connector domain.ConnectorKey, operation string) (ports.PartnerReader, error) {
	reader, err := c.PartnerAdapters.Reader(connector, operation)
	if err != nil {
		return nil, err
	}
	return countedReader{PartnerReader: reader, counted: c}, nil
}

func (c *countedReads) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

type countedReader struct {
	ports.PartnerReader
	counted *countedReads
}

func (r countedReader) Read(ctx context.Context, parameters []byte) (ports.RawExchange, error) {
	r.counted.mu.Lock()
	r.counted.n++
	r.counted.mu.Unlock()
	return r.PartnerReader.Read(ctx, parameters)
}

// reading is the dispatching harness whose connector reads queryAsset: enabled
// by the Partner, declared in the profile in force with a reading of at most
// maxPayload bytes, and authorized for org-1 and duplicata.
type reading struct {
	*dispatching
	reads *countedReads
}

func newReading(t *testing.T, maxPayload int) *reading {
	t.Helper()
	d := newDispatching(t, nil)
	d.enabled(t, "queryAsset", false, 1)
	queryAsset := simulatorOperation("queryAsset")
	queryAsset.MaxPayloadBytes = maxPayload
	must(d.service.DeclareCapabilityProfile(withExecution(t), application.DeclareCapabilityProfile{
		ConnectorKey: testConnector, Version: 3, EffectiveFrom: testOccurred,
		Operations: []domain.OperationCapability{simulatorOperation("submitEffect"), queryAsset},
	})).accepted(t)
	must(d.service.GrantQueryAuthorization(withExecution(t), grantAuthorization())).accepted(t)
	reads := &countedReads{PartnerAdapters: d.partners}
	d.service.Partners = reads
	return &reading{dispatching: d, reads: reads}
}

func readAsset(read partnersim.Read) application.ReadExternal {
	return application.ReadExternal{
		ConnectorKey: testConnector, Operation: "queryAsset", OrganizationID: "org-1", AssetType: "duplicata",
		Parameters: partnersim.Parameters(read),
	}
}

var position = partnersim.Read{ExternalRef: "ast-ext-1", Reading: []byte(`{"owner":"org-1","position":100}`)}

func (r *reading) readID(t *testing.T, key string) domain.InteractionID {
	t.Helper()
	id, err := r.service.DerivedKeys.ExternalReadInteractionID(testConnector, key)
	if err != nil {
		t.Fatalf("ExternalReadInteractionID() = %v", err)
	}
	return id
}

// nothingRead tells that no read reached the partner and nothing was kept.
func (r *reading) nothingRead(t *testing.T, key string) {
	t.Helper()
	if calls, versions := r.reads.calls(), r.raw.versions(ports.RawKeyOf(testConnector, r.readID(t, key), ports.RawRequest)); calls != 0 || versions != 0 {
		t.Fatalf("reads = %d, versions = %d; want no call to the partner", calls, versions)
	}
}

// L902-904: the answer carries the provenance and the normalized reading, and
// the external_read interaction is recorded with both raw sides in the store.
func TestReadExternalAnswersTheReadingAndRecordsTheExternalRead(t *testing.T) {
	r := newReading(t, 1<<12)

	got := must(r.service.ReadExternal(withKey(t, "read-1"), readAsset(position))).accepted(t)

	id := r.readID(t, "read-1")
	if got.InteractionID != id || got.AsOf != testOccurred || got.ProviderID != "prt-1" || got.ExternalReference != "ast-ext-1" ||
		!strings.Contains(got.Reading, `"position":100`) || got.ReadingHash == "" || got.Outcome != domain.InteractionNormalized {
		t.Fatalf("ReadExternal() = %+v, want the reading of %s with its provenance", got, id)
	}
	i := r.interaction(t, id)
	if i.Direction != domain.DirectionOutbound || i.Source != domain.InteractionExternalRead || i.Operation != "queryAsset" ||
		i.CommandID != "" || i.Status != domain.InteractionStored || i.Request.VersionID != "v1" || i.Response.VersionID != "v1" ||
		i.ContentType != partnersim.ContentType {
		t.Fatalf("interaction = %+v, want the external read with both raw sides", i)
	}
	handovers := r.handovers()
	r.normalized(t, "n-r", handovers[len(handovers)-1], usecase.R1D1)
	if normalized := r.interaction(t, id); normalized.Status != domain.InteractionNormalized || normalized.ReadingHash != got.ReadingHash {
		t.Fatalf("normalized interaction = %+v, want the reading hash the answer carried", normalized)
	}
}

// A replay of the key calls the partner again, keeps the second answer as an
// orphan version and answers the reading recorded the first time.
func TestAReplayOfTheReadCallsThePartnerAgainAndAnswersTheFirstReading(t *testing.T) {
	r := newReading(t, 1<<12)
	first := must(r.service.ReadExternal(withKey(t, "read-1"), readAsset(position))).accepted(t)
	r.clock.advance(5 * second)

	again := must(r.service.ReadExternal(withKey(t, "read-1"), readAsset(position))).accepted(t)

	if again != first {
		t.Fatalf("replay = %+v, want the first reading %+v", again, first)
	}
	if calls, versions := r.reads.calls(), r.raw.versions(ports.RawKeyOf(testConnector, first.InteractionID, ports.RawResponse)); calls != 2 || versions != 2 {
		t.Fatalf("reads = %d, versions = %d; want the partner called again and an orphan version", calls, versions)
	}
	if handovers := r.handovers(); len(handovers) != 1 {
		t.Fatalf("handovers = %+v, want the read recorded once", handovers)
	}
}

// L606-612: the reads of ReceivablesQueryPort fail closed without a granted
// authorization for the organization and the asset type.
func TestAReadOfReceivablesQueryPortFailsClosedWithoutAnActiveAuthorization(t *testing.T) {
	tests := map[string]func(*testing.T, *reading) application.ReadExternal{
		"revoked": func(t *testing.T, r *reading) application.ReadExternal {
			must(r.service.RevokeQueryAuthorization(withExecution(t), application.RevokeQueryAuthorization{ConsentID: "cns-1", EffectiveAt: testOccurred})).accepted(t)
			return readAsset(position)
		},
		"another organization": func(*testing.T, *reading) application.ReadExternal {
			read := readAsset(position)
			read.OrganizationID = "org-2"
			return read
		},
		"another asset type": func(*testing.T, *reading) application.ReadExternal {
			read := readAsset(position)
			read.AssetType = "cartao"
			return read
		},
	}
	for name, arrange := range tests {
		t.Run(name, func(t *testing.T) {
			r := newReading(t, 1<<12)
			read := arrange(t, r)

			must(r.service.ReadExternal(withKey(t, "read-1"), read)).refused(t, string(domain.CodeQueryAuthorizationConsentNotActive))

			r.nothingRead(t, "read-1")
		})
	}
}

// L611-613: discoverSchedule reads the query port too, so once the
// authorization is revoked it fails closed before any call.
func TestDiscoverScheduleFailsClosedOnceTheAuthorizationIsRevoked(t *testing.T) {
	r := newReading(t, 1<<12)
	r.enabled(t, "discoverSchedule", false, 1)
	queryAsset, discover := simulatorOperation("queryAsset"), simulatorOperation("discoverSchedule")
	queryAsset.MaxPayloadBytes, discover.MaxPayloadBytes = 1<<12, 1<<12
	must(r.service.DeclareCapabilityProfile(withExecution(t), application.DeclareCapabilityProfile{
		ConnectorKey: testConnector, Version: 4, EffectiveFrom: testOccurred,
		Operations: []domain.OperationCapability{simulatorOperation("submitEffect"), queryAsset, discover},
	})).accepted(t)
	must(r.service.RevokeQueryAuthorization(withExecution(t), application.RevokeQueryAuthorization{ConsentID: "cns-1", EffectiveAt: testOccurred})).accepted(t)
	read := readAsset(position)
	read.Operation = "discoverSchedule"

	must(r.service.ReadExternal(withKey(t, "read-1"), read)).refused(t, string(domain.CodeQueryAuthorizationConsentNotActive))

	r.nothingRead(t, "read-1")
}

// L614-616: a read the profile does not support, or of an operation the
// connector has suspended or never had enabled, is refused before any call.
func TestAReadOfAnOperationNotEnabledIsRefusedWithoutCallingThePartner(t *testing.T) {
	tests := map[string]func(*testing.T, *reading) application.ReadExternal{
		"suspended": func(t *testing.T, r *reading) application.ReadExternal {
			must(r.service.SuspendOperation(withExecution(t), application.SuspendOperation{
				ConnectorKey: testConnector, Operation: "queryAsset", Cause: domain.CauseSchemaMismatch,
			})).accepted(t)
			return readAsset(position)
		},
		"not declared": func(*testing.T, *reading) application.ReadExternal {
			read := readAsset(position)
			read.Operation = "queryOwnership"
			return read
		},
		"not enabled": func(*testing.T, *reading) application.ReadExternal {
			read := readAsset(position)
			read.Operation = "queryEffect"
			return read
		},
	}
	for name, arrange := range tests {
		t.Run(name, func(t *testing.T) {
			r := newReading(t, 1<<12)
			read := arrange(t, r)
			if read.Operation == "queryEffect" {
				must(r.service.DeclareCapabilityProfile(withExecution(t), application.DeclareCapabilityProfile{
					ConnectorKey: testConnector, Version: 4, EffectiveFrom: testOccurred,
					Operations: []domain.OperationCapability{simulatorOperation("queryEffect")},
				})).accepted(t)
			}

			must(r.service.ReadExternal(withKey(t, "read-1"), read)).refused(t, string(domain.CodeConnectorCapabilityNotEnabled))

			r.nothingRead(t, "read-1")
		})
	}
}

// Decision 21: with no adapter for the connector the read fails closed as a
// dependency, before any call.
func TestAReadWithoutPartnerAdapterFailsAsADependency(t *testing.T) {
	r := newReading(t, 1<<12)
	r.service.Partners = ports.NoPartnerAdapters{}

	_, err := r.service.ReadExternal(withKey(t, "read-1"), readAsset(position))

	if failure, ok := errors.AsType[*usecase.Failure](err); !ok || failure.Category() != usecase.TransientDependency ||
		!errors.Is(err, ports.ErrPartnerAdapterUnavailable) {
		t.Fatalf("ReadExternal() = %v, want an unavailable dependency", err)
	}
	if n := r.raw.versions(ports.RawKeyOf(testConnector, r.readID(t, "read-1"), ports.RawRequest)); n != 0 {
		t.Fatalf("versions = %d, want nothing kept", n)
	}
}

// Decision 20: the outcome the inbox keeps holds a reading of at most
// maxPayloadBytes; a larger one is refused, and the read stays recorded.
func TestAReadingAboveMaxPayloadBytesIsRefusedAndTheReadStaysRecorded(t *testing.T) {
	r := newReading(t, 32)

	must(r.service.ReadExternal(withKey(t, "read-1"), readAsset(position))).refused(t, string(domain.CodeExternalInteractionPayloadTooLarge))
	must(r.service.ReadExternal(withKey(t, "read-1"), readAsset(position))).refused(t, string(domain.CodeExternalInteractionPayloadTooLarge))

	if i := r.interaction(t, r.readID(t, "read-1")); i.Source != domain.InteractionExternalRead || i.Status != domain.InteractionStored {
		t.Fatalf("interaction = %+v, want the read recorded", i)
	}
}

// A reading outside the partner's contract is recorded for the normalization,
// which detects the mismatch, and the caller gets a failure, now and on replay.
func TestAReadingOutsideTheContractIsRecordedAndFails(t *testing.T) {
	r := newReading(t, 1<<12)
	broken := position
	broken.Break = domain.MismatchMissingField

	for range 2 {
		_, err := r.service.ReadExternal(withKey(t, "read-1"), readAsset(broken))
		if !errors.Is(err, application.ErrReadingNotNormalized) {
			t.Fatalf("ReadExternal() = %v, want the reading refused as not normalized", err)
		}
	}
	if handovers := r.handovers(); len(handovers) != 1 || handovers[0].InteractionID != r.readID(t, "read-1") {
		t.Fatalf("handovers = %+v, want the read handed to the normalization once", handovers)
	}
}

func TestAReadOutsideTheVocabularyOfThePortsIsInvalid(t *testing.T) {
	r := newReading(t, 1<<12)
	read := readAsset(position)
	read.Operation = "submitEffect"

	_, err := r.service.ReadExternal(withKey(t, "read-1"), read)

	if failure, ok := errors.AsType[*usecase.Failure](err); !ok || failure.Category() != usecase.Validation {
		t.Fatalf("ReadExternal(submitEffect) = %v, want a validation failure", err)
	}
	r.nothingRead(t, "read-1")
}
