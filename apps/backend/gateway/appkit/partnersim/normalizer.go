package partnersim

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

// ErrNothingToNormalize is an interaction without the side its source reads.
var ErrNothingToNormalize = errors.New("partnersim: the interaction carries no body to read")

// Normalizer reads the simulator's format. It is pure over the bytes, so one
// raw payload always reads one way, whichever time it is read.
type Normalizer struct{}

var _ ports.InteractionNormalizer = Normalizer{}

// Normalize reads the partner's side: the request of what the partner sent on
// its own, the response of what the context asked. A body that names no
// command is uncorrelated, unless it answers an external read, which names none.
func (Normalizer) Normalize(source domain.InteractionSource, exchange ports.RawExchange) (ports.Reading, error) {
	raw := exchange.Response.Body
	if source == domain.InteractionCallback || source == domain.InteractionFile {
		raw = exchange.Request.Body
	}
	if len(raw) == 0 {
		return ports.Reading{}, fmt.Errorf("%w: %s", ErrNothingToNormalize, source)
	}
	p, broke := parse(raw)
	if broke != nil {
		return ports.Reading{Outcome: domain.InteractionSchemaMismatch, Mismatch: *broke}, nil
	}
	reading := ports.Reading{
		CommandKey: port.OutboundKey(p.outboundKey), Finding: p.finding, ExternalRef: p.externalRef,
		AsOf: port.Instant(p.at), References: links(p),
	}
	if p.outboundKey == "" && source != domain.InteractionExternalRead {
		reading.Outcome = domain.InteractionUncorrelated
		return reading, nil
	}
	reading.Outcome = domain.InteractionNormalized
	reading.Canonical = canonical(p)
	sum := sha256.Sum256(reading.Canonical)
	reading.ReadingHash = hex.EncodeToString(sum[:])
	return reading, nil
}

func links(p parsed) []domain.LinkExternalReference {
	var out []domain.LinkExternalReference
	for _, r := range p.references {
		out = append(out, domain.LinkExternalReference{
			ProviderID: domain.PartnerID(r.ProviderID), System: domain.ExternalSystem(r.System), ReferenceType: domain.ReferenceType(r.ReferenceType),
			Reference: r.Reference, NormalizedReference: r.NormalizedReference, SubjectType: r.SubjectType, SubjectID: domain.SubjectID(r.SubjectID),
			AsOf: port.Instant(p.at), ObservedVersion: r.ObservedVersion,
		})
	}
	return out
}

// canonical fixes the bytes of a reading: the fields in declared order, the
// keys of the free reading sorted and its numbers as written, so the hash is
// the same for the same reading.
func canonical(p parsed) []byte {
	out, err := json.Marshal(struct {
		Status      int         `json:"status"`
		CommandKey  string      `json:"commandKey,omitempty"`
		Finding     string      `json:"finding,omitempty"`
		ExternalRef string      `json:"externalRef,omitempty"`
		AsOf        int64       `json:"asOf,omitempty"`
		References  []Reference `json:"references,omitempty"`
		Reading     any         `json:"reading,omitempty"`
	}{p.status, p.outboundKey, findingName(p.finding), p.externalRef, p.at, p.references, p.reading})
	if err != nil {
		panic(err)
	}
	return out
}
