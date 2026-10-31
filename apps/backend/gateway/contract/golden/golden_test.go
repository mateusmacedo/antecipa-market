package golden

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	contractRoot       = "apps/backend/gateway/contract/"
	fixtureSource      = "urn:dmpf:reference-gateway"
	unknownFieldNumber = 99
)

var specs = []tb.Spec[proto.Message]{
	externalCommandSubmittedSpec, externalCommandAcknowledgedSpec, externalCommandConfirmedSpec,
	externalCommandFailedSpec, externalOutcomeUnknownSpec, externalCommandAmbiguityExpiredSpec,
	externalCommandDispatchRequestedSpec,
}

// tb.GoldenSuite and tb.UpdateGolden resolve fixtures under tb.RepoRoot, the go.work
// above the testkit source (testkit v1.0.0-rc.7 tb/tb.go); a module-cache copy
// has none, so this suite runs the same oracles over fixtures read from here.
func TestGolden(t *testing.T) {
	each := func(name string, body func(t *testing.T, s tb.Spec[proto.Message])) {
		t.Run(name, func(t *testing.T) {
			for _, s := range specs {
				t.Run(s.Identity.Fixture, func(t *testing.T) { body(t, s) })
			}
		})
	}

	each("FixtureIsInSyncWithGenerator", fixtureIsInSyncWithGenerator)
	each("FixtureRejectsUnknownFormatVersion", fixtureRejectsUnknownFormatVersion)
	t.Run("OneFixturePerContractMajor", oneFixturePerContractMajor)
	each("FixtureShape", func(t *testing.T, s tb.Spec[proto.Message]) { requireShape(t, load(t, s), s) })
	each("RoundTrip", roundTrip)
	each("UnknownFieldIsPreservedAndHashed", unknownFieldIsPreservedAndHashed)
	each("HashOverBytesNotOverStructure", hashOverBytesNotOverStructure)
}

func TestUpdateGolden(t *testing.T) {
	if os.Getenv("GOLDEN_UPDATE") != "1" {
		return
	}
	for _, s := range specs {
		t.Run(s.Identity.Fixture, func(t *testing.T) {
			if err := os.WriteFile(localPath(t, s), append(indented(t, s.Build(t, s)), '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func fixtureIsInSyncWithGenerator(t *testing.T, s tb.Spec[proto.Message]) {
	if !bytes.Equal(indented(t, load(t, s)), indented(t, s.Build(t, s))) {
		t.Fatal("committed fixture differs from the generator; run GOLDEN_UPDATE=1 go test -run TestUpdateGolden and review the diff")
	}
}

func fixtureRejectsUnknownFormatVersion(t *testing.T, s tb.Spec[proto.Message]) {
	doc := s.Build(t, s)
	doc.FormatVersion = "2"
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := golden.Decode(data); !errors.Is(err, golden.ErrFormatVersion) {
		t.Fatalf("err = %v, want golden.ErrFormatVersion", err)
	}
}

func oneFixturePerContractMajor(t *testing.T) {
	var catalog golden.Catalog
	for _, s := range specs {
		if err := catalog.Add(s.Path, load(t, s)); err != nil {
			t.Fatal(err)
		}
	}
	if catalog.Len() != len(specs) {
		t.Fatalf("catalog holds %d fixtures, want %d", catalog.Len(), len(specs))
	}
}

func roundTrip(t *testing.T, s tb.Spec[proto.Message]) {
	doc := load(t, s)
	report := golden.Evaluate(doc, golden.Subject{NewMessage: s.New, MessageFromFields: s.FromFields})
	if want := 3*len(doc.AllCases()) + 3*len(doc.Cases); len(report.Outcomes) != want {
		t.Fatalf("%d outcomes, want %d (three oracles per direction)", len(report.Outcomes), want)
	}
	tb.RequireReport(t, report)
}

func unknownFieldIsPreservedAndHashed(t *testing.T, s tb.Spec[proto.Message]) {
	c := requireCase(t, load(t, s), "unknown-field")
	transported := decodeHex(t, c.PayloadBytesHex)
	if len(unmarshal(t, s, transported).ProtoReflect().GetUnknown()) == 0 {
		t.Fatalf("unknown field %d was dropped on decode (PTB-10)", s.Unknown)
	}
	if payloadhash.Sum(transported) != c.PayloadHash {
		t.Fatal("payload_hash must cover the unknown field bytes (ENV-17)")
	}
}

func hashOverBytesNotOverStructure(t *testing.T, s tb.Spec[proto.Message]) {
	c := requireCase(t, load(t, s), "non-canonical-field-order")
	transported := decodeHex(t, c.PayloadBytesHex)
	if payloadhash.Sum(transported) != c.PayloadHash {
		t.Fatal("Sum(transported) must equal the declared payload_hash")
	}
	decoded := unmarshal(t, s, transported)
	if !proto.Equal(decoded, s.Read(t, c.Payload)) {
		t.Fatalf("non-canonical bytes decoded to %v", decoded)
	}
	reserialized, err := proto.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if payloadhash.Sum(reserialized) == c.PayloadHash {
		t.Fatal("hash of the reserialized structure must differ from the transported hash (ENV-18)")
	}
}

func localPath(t *testing.T, s tb.Spec[proto.Message]) string {
	t.Helper()
	rel, ok := strings.CutPrefix(s.Path, contractRoot)
	if !ok {
		t.Fatalf("fixture %s lies outside %s", s.Path, contractRoot)
	}
	return filepath.Join("..", filepath.FromSlash(rel))
}

func load(t *testing.T, s tb.Spec[proto.Message]) golden.Fixture {
	t.Helper()
	raw, err := os.ReadFile(localPath(t, s))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	doc, err := golden.Decode(raw)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return doc
}

func indented(t *testing.T, doc golden.Fixture) []byte {
	t.Helper()
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func requireShape(t *testing.T, doc golden.Fixture, s tb.Spec[proto.Message]) {
	t.Helper()
	if doc.Identity != s.Identity {
		t.Fatalf("identity = %+v, want %+v", doc.Identity, s.Identity)
	}
	if doc.Covers.ProfileMajor != "1" || doc.Covers.ContractMajor != "v1" {
		t.Fatalf("covers = %+v", doc.Covers)
	}
	if len(doc.Cases) != s.WantCases || len(doc.Discriminators) != s.WantDiscriminators {
		t.Fatalf("cases = %d, discriminators = %d; want %d and %d", len(doc.Cases), len(doc.Discriminators), s.WantCases, s.WantDiscriminators)
	}
	for _, name := range []string{"aggregateversion", "tenantid", "tracestate"} {
		present, absent := 0, 0
		for _, c := range doc.AllCases() {
			if _, ok := c.Envelope[name]; ok {
				present++
			} else {
				absent++
			}
		}
		if present == 0 || absent == 0 {
			t.Errorf("conditional %q lacks a case in one of the two states (present=%d absent=%d)", name, present, absent)
		}
	}
	values := map[string]bool{}
	for _, c := range doc.AllCases() {
		values[c.Payload[s.Enum.Field]] = true
	}
	for _, want := range s.Enum.Values {
		if !values[want] {
			t.Errorf("enum discriminator %q missing on field %q (ORA-08)", want, s.Enum.Field)
		}
	}
}

func requireCase(t *testing.T, doc golden.Fixture, name string) golden.Case {
	t.Helper()
	c, ok := doc.FindCase(name)
	if !ok {
		t.Fatalf("case %q not in fixture", name)
	}
	return c
}

func decodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("payload_bytes_hex: %v", err)
	}
	return b
}

func unmarshal(t *testing.T, s tb.Spec[proto.Message], transported []byte) proto.Message {
	t.Helper()
	msg := s.New()
	if err := proto.Unmarshal(transported, msg); err != nil {
		t.Fatalf("payload does not decode: %v", err)
	}
	return msg
}

func parseInstant(fields map[string]string, name string) (*timestamppb.Timestamp, error) {
	at, err := time.Parse(time.RFC3339Nano, fields[name])
	if err != nil {
		return nil, fmt.Errorf("%s %q: %w", name, fields[name], err)
	}
	return timestamppb.New(at), nil
}

func leanFixture(present, absent, unknown, nonCanonical map[string]string) func(*testing.T, tb.Spec[proto.Message]) golden.Fixture {
	return func(t *testing.T, s tb.Spec[proto.Message]) golden.Fixture {
		t.Helper()

		allPresent := s.PackedCase(t, "all-conditionals-present",
			"Caminho típico com os três atributos condicionais presentes (aggregateversion, tenantid, tracestate).",
			tb.WithConditionals(s.BaseEnvelope()), present)

		absentEnv := s.BaseEnvelope()
		absentEnv["id"] = "evt-0002"
		allAbsent := s.PackedCase(t, "all-conditionals-absent",
			"Os três condicionais ausentes: nenhum entra no mapa de atributos (ENV-12, sem valor de preenchimento).",
			absentEnv, absent)

		unknownEnv := s.BaseEnvelope()
		unknownEnv["id"] = "evt-2001"
		unknownField := s.UnknownFieldCase(t, "unknown-field",
			fmt.Sprintf("Bytes canônicos mais um campo de número %d (varint 42) que o contrato não conhece: a desserialização o preserva (PTB-10) e o hash o cobre (ENV-17).", s.Unknown),
			unknownEnv, unknown)

		nonCanonicalEnv := s.BaseEnvelope()
		nonCanonicalEnv["id"] = "evt-2002"
		nonCanonicalCase := s.NonCanonicalCase(t, "non-canonical-field-order",
			"Campos em ordem decrescente de número, o instante como sub-mensagem: decodifica no mesmo valor, mas o hash é o dos bytes transportados e difere do hash de uma reserialização (ENV-18). Campo repetido fica vazio e booleano fica falso, porque o codificador não canônico do kit cobre só strings, inteiros, enums e mensagens.",
			nonCanonicalEnv, nonCanonical)

		return s.Fixture(
			[]golden.Case{allPresent, allAbsent},
			[]golden.Case{unknownField, nonCanonicalCase},
		)
	}
}

func parseBool(fields map[string]string, name string) (bool, error) {
	b, err := strconv.ParseBool(fields[name])
	if err != nil {
		return false, fmt.Errorf("%s %q: %w", name, fields[name], err)
	}
	return b, nil
}

func parseInt32(fields map[string]string, name string) (int32, error) {
	n, err := tb.ParseInt(fields, name, 32)
	return int32(n), err
}

func enumOf(values map[string]int32, fields map[string]string, name string) (int32, error) {
	value, ok := values[fields[name]]
	if !ok {
		return 0, fmt.Errorf("%s %q is not a value of the enum", name, fields[name])
	}
	return value, nil
}

func withEnumCoverage(build func(*testing.T, tb.Spec[proto.Message]) golden.Fixture, extra ...map[string]string) func(*testing.T, tb.Spec[proto.Message]) golden.Fixture {
	return func(t *testing.T, s tb.Spec[proto.Message]) golden.Fixture {
		t.Helper()
		doc := build(t, s)
		for i, fields := range extra {
			env := s.BaseEnvelope()
			env["id"] = fmt.Sprintf("evt-%04d", 3+i)
			doc.Cases = append(doc.Cases, s.PackedCase(t, fmt.Sprintf("enum-coverage-%d", i+1),
				fmt.Sprintf("Caso empacotado que cobre o valor %s do campo %s, que os quatro casos da fixture enxuta não alcançam.", fields[s.Enum.Field], s.Enum.Field),
				env, fields))
		}
		return doc
	}
}
