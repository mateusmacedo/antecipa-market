package partnersim

import (
	"bytes"
	"encoding/json"
	"fmt"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

// Format is the version the simulator declares in every body it writes; a
// body under another one breaks the contract as a version mismatch.
const Format = "partnersim/v1"

// ContentType is the type of every raw payload of the simulator.
const ContentType = "application/json"

// Fault is a call that brought no response back: refused before the partner
// read it, or lost after the partner may have acted on it.
type Fault string

const (
	FaultRefused Fault = "refused"
	FaultLost    Fault = "lost"
)

// Directive is the script of one command, under "simulator" in its payload:
// each submission of the command takes the next reply, each state query of
// it the next answer.
type Directive struct {
	Replies []Reply  `json:"replies,omitempty"`
	Queries []Answer `json:"queries,omitempty"`
}

// Reply is what the partner answers to one submission. An accepted reply
// applies the effect, and Applied marks it on one that says otherwise, as a
// response lost after the effect; Break sends a body that breaks the contract
// in that way instead of a well-formed one.
type Reply struct {
	Status      int                 `json:"status,omitempty"`
	Fault       Fault               `json:"fault,omitempty"`
	Applied     bool                `json:"applied,omitempty"`
	Finding     string              `json:"finding,omitempty"`
	ExternalRef string              `json:"externalRef,omitempty"`
	References  []Reference         `json:"references,omitempty"`
	Break       domain.MismatchKind `json:"break,omitempty"`
}

// Answer is what the partner answers to one state query; FromEffect answers
// with the effect the submissions left: applied or absent.
type Answer struct {
	Finding     string              `json:"finding,omitempty"`
	FromEffect  bool                `json:"fromEffect,omitempty"`
	ExternalRef string              `json:"externalRef,omitempty"`
	Fault       Fault               `json:"fault,omitempty"`
	Break       domain.MismatchKind `json:"break,omitempty"`
}

// Read is the script of one external read, under "simulator" in its
// parameters: the partner answers it with the reading as given.
type Read struct {
	ExternalRef string              `json:"externalRef,omitempty"`
	Reading     json.RawMessage     `json:"reading,omitempty"`
	References  []Reference         `json:"references,omitempty"`
	Fault       Fault               `json:"fault,omitempty"`
	Break       domain.MismatchKind `json:"break,omitempty"`
}

// Notice is a message the partner sends on its own, a callback or a file;
// without Key it names no command and reads as uncorrelated.
type Notice struct {
	Key         port.OutboundKey
	Finding     port.Finding
	ExternalRef string
	References  []Reference
	Break       domain.MismatchKind
}

// Reference is an external reference a body carries, linked as of the
// instant of the body.
type Reference struct {
	ProviderID          string `json:"providerId"`
	System              string `json:"system"`
	ReferenceType       string `json:"referenceType"`
	Reference           string `json:"reference"`
	NormalizedReference string `json:"normalizedReference,omitempty"`
	SubjectType         string `json:"subjectType"`
	SubjectID           string `json:"subjectId"`
	ObservedVersion     string `json:"observedVersion,omitempty"`
}

// Payload is a command payload carrying the directive, beside the business
// data a real partner would read.
func Payload(directive Directive) []byte {
	return mustMarshal(map[string]any{"simulator": directive})
}

// Parameters are the parameters of an external read carrying its script.
func Parameters(read Read) []byte {
	return mustMarshal(map[string]any{"simulator": read})
}

func scriptOf[T any](raw []byte) (T, bool) {
	var envelope struct {
		Simulator *T `json:"simulator"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Simulator == nil {
		var zero T
		return zero, false
	}
	return *envelope.Simulator, true
}

// body is the well-formed shape of every response and notice of the format.
type body struct {
	Format        string           `json:"format"`
	Status        int              `json:"status"`
	OutboundKey   string           `json:"outboundKey,omitempty"`
	Finding       string           `json:"finding,omitempty"`
	ExternalRef   string           `json:"externalRef,omitempty"`
	ObservationID string           `json:"observationId,omitempty"`
	At            int64            `json:"at,omitempty"`
	References    []Reference      `json:"references,omitempty"`
	Reading       *json.RawMessage `json:"reading,omitempty"`
}

// encode writes the body, broken in the way asked: the four kinds of 04 §5.
func encode(b body, broken domain.MismatchKind) []byte {
	b.Format = Format
	switch broken {
	case domain.MismatchStructure:
		return mustMarshal([]any{b.Format, b.Status})
	case domain.MismatchVersion:
		b.Format = "partnersim/v9"
	}
	fields := map[string]any{}
	decoder := json.NewDecoder(bytes.NewReader(mustMarshal(b)))
	decoder.UseNumber()
	if err := decoder.Decode(&fields); err != nil {
		panic(err)
	}
	switch broken {
	case domain.MismatchMissingField:
		delete(fields, "status")
	case domain.MismatchType:
		fields["status"] = fmt.Sprint(b.Status)
	}
	return mustMarshal(fields)
}

func request(operation string, fields map[string]any) []byte {
	fields["format"], fields["operation"] = Format, operation
	return mustMarshal(fields)
}

// asJSON embeds a payload as it is when it is JSON, as a string otherwise.
func asJSON(raw []byte) any {
	if json.Valid(raw) {
		return json.RawMessage(raw)
	}
	return string(raw)
}

// parsed is a body that keeps the contract.
type parsed struct {
	status      int
	outboundKey string
	finding     port.Finding
	externalRef string
	at          int64
	references  []Reference
	reading     any
}

// parse reads a body of the format. An unknown finding is no break of the
// contract (04 §5): it reads as inconclusive.
func parse(raw []byte) (parsed, *domain.Mismatch) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return parsed{}, &domain.Mismatch{Kind: domain.MismatchStructure, Detail: "the body is not a JSON object"}
	}
	var format string
	if broke := field(fields, "format", &format, true); broke != nil {
		return parsed{}, broke
	}
	if format != Format {
		return parsed{}, &domain.Mismatch{Kind: domain.MismatchVersion, Detail: "format " + format + " is not " + Format, PartnerDeclaredVersion: format}
	}
	var p parsed
	var finding string
	for _, f := range []struct {
		name     string
		into     any
		required bool
	}{
		{"status", &p.status, true}, {"outboundKey", &p.outboundKey, false}, {"finding", &finding, false},
		{"externalRef", &p.externalRef, false}, {"at", &p.at, false}, {"references", &p.references, false},
	} {
		if broke := field(fields, f.name, f.into, f.required); broke != nil {
			return parsed{}, broke
		}
	}
	p.finding = findingOf(finding)
	if reading, ok := fields["reading"]; ok {
		decoder := json.NewDecoder(bytes.NewReader(reading))
		decoder.UseNumber()
		if err := decoder.Decode(&p.reading); err != nil {
			return parsed{}, &domain.Mismatch{Kind: domain.MismatchType, Detail: "reading is not JSON"}
		}
	}
	return p, nil
}

func field(fields map[string]json.RawMessage, name string, into any, required bool) *domain.Mismatch {
	raw, ok := fields[name]
	if !ok {
		if required {
			return &domain.Mismatch{Kind: domain.MismatchMissingField, Detail: name + " is missing"}
		}
		return nil
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return &domain.Mismatch{Kind: domain.MismatchType, Detail: name + " has another type"}
	}
	return nil
}

func findingOf(name string) port.Finding {
	switch name {
	case "":
		return 0
	case "applied":
		return port.FindingApplied
	case "absent":
		return port.FindingAbsent
	case "rejected":
		return port.FindingRejected
	case "pending":
		return port.FindingPending
	default:
		return port.FindingInconclusive
	}
}

func findingName(f port.Finding) string {
	if f == 0 {
		return ""
	}
	return f.String()
}

func mustMarshal(v any) []byte {
	out, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return out
}
