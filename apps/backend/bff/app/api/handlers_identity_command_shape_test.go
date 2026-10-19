package api_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

func capitalized(name string) string {
	first, size := utf8.DecodeRuneInString(name)
	return string(unicode.ToUpper(first)) + name[size:]
}

func renamed(field string) edit {
	return func(m map[string]any) {
		m[capitalized(field)] = m[field]
		delete(m, field)
	}
}

func repeatedMember(t *testing.T, base map[string]any, field string) string {
	t.Helper()
	value, err := json.Marshal(base[field])
	if err != nil {
		t.Fatalf("Marshal(%v) = %v", base[field], err)
	}
	body := bodyWith(t, base)
	return strings.TrimSuffix(body, "}") + `,"` + field + `":` + string(value) + "}"
}

func TestEveryKeyedIdentityRouteRefusesABodyOutsideTheExactMembersOfItsSchema(t *testing.T) {
	for _, route := range keyedRoutes() {
		if route.body == nil {
			continue
		}
		var bounds []bound
		for _, field := range slices.Sorted(maps.Keys(route.body)) {
			bounds = append(bounds,
				bound{field + " in another case", bodyWith(t, route.body, renamed(field)), nil},
				bound{field + " repeated", repeatedMember(t, route.body, field), nil},
				bound{field + " null", bodyWith(t, route.body, with(field, nil)), nil},
			)
		}
		t.Run(route.name, func(t *testing.T) { requireBoundsOf(t, route, bounds) })
	}
}

func TestANestedMemberIsBoundByTheSameExactShape(t *testing.T) {
	reference := func(edits ...edit) edit {
		return with("externalReferences", []any{cloned(t, externalReference(), edits...)})
	}
	body := func(edits ...edit) string { return bodyWith(t, establishmentBody(), edits...) }
	repeatedInside := strings.Replace(body(), `"reference":"ref-1"`, `"reference":"ref-1","reference":"ref-1"`, 1)
	requireBounds(t, "addEstablishment", []bound{
		{"the exact nested members", body(), reaches},
		{"a nested member in another case", body(reference(renamed("reference"))), nil},
		{"a nested member repeated", repeatedInside, nil},
		{"an optional nested member null", body(reference(with("observedVersion", nil))), nil},
		{"a required nested member null", body(reference(with("asOf", nil))), nil},
		{"a null item", body(with("externalReferences", []any{nil})), nil},
	})
}

func TestAnOptionalMemberIsAcceptedAbsentButNeverNull(t *testing.T) {
	cases := []struct {
		route string
		field string
	}{
		{"registerOrganization", "tradeName"},
		{"addEstablishment", "externalReferences"},
		{"provisionUserAccount", "personId"},
		{"registerRepresentation", "validUntil"},
		{"registerRepresentation", "scopeCurrency"},
		{"registerRepresentation", "scopeMaxAmountMinor"},
		{"registerRepresentation", "scopeActTypes"},
	}
	for _, tc := range cases {
		t.Run(tc.route+"/"+tc.field, func(t *testing.T) {
			base := commandNamed(t, tc.route).body
			requireBounds(t, tc.route, []bound{
				{"absent", bodyWith(t, base, without(tc.field)), reaches},
				{"null", bodyWith(t, base, with(tc.field, nil)), nil},
			})
		})
	}
}

func TestABodyThatIsNotUnicodeTextIsRefused(t *testing.T) {
	evidence := func(text string) string {
		return `{"bankAccountChangeId":"bac-1","destinationEvidenceId":"ev` + text + `"}`
	}
	evidenceOf := func(want string) func(proto.Message) bool {
		return routedAs(func(r *servicev1.RequestBankAccountChangeRequest) bool { return r.GetDestinationEvidenceId() == want })
	}
	requireBounds(t, "requestBankAccountChange", []bound{
		{"an escaped replacement character", evidence(`\ufffd`), evidenceOf("ev�")},
		{"an escaped surrogate pair", evidence(`\ud83d\ude00`), evidenceOf("ev\U0001F600")},
		{"an escaped backslash before u", evidence(`\\ud800`), evidenceOf(`ev\ud800`)},
		{"an invalid UTF-8 byte", evidence("\xff"), nil},
		{"a truncated UTF-8 sequence", evidence("\xc3"), nil},
		{"an encoded surrogate", evidence("\xed\xa0\x80"), nil},
		{"an unpaired high surrogate", evidence(`\ud800`), nil},
		{"an unpaired low surrogate", evidence(`\udc00`), nil},
		{"a high surrogate before another escape", evidence(`\ud800A`), nil},
		{"two high surrogates", evidence(`\ud800\ud800`), nil},
		{"an unpaired surrogate in a member name", `{"bankAccountChangeId":"bac-1","destinationEvidenceId":"ev","\ud800":"x"}`, nil},
	})
	requireBounds(t, "addEstablishment", []bound{
		{"an unpaired surrogate in a nested member", strings.Replace(bodyWith(t, establishmentBody()), `"ref-1"`, `"ref-1\udfff"`, 1), nil},
	})
}

func answeredWithID(t *testing.T, command identityCommand, id string) reply {
	t.Helper()
	answered, err := command.reply(nil)
	if err != nil {
		t.Fatalf("reply of %s = %v", command.name, err)
	}
	message := proto.Clone(answered.(proto.Message)).ProtoReflect()
	result := message.WhichOneof(message.Descriptor().Oneofs().ByName("result"))
	inner := message.Mutable(result).Message()
	inner.Set(inner.Descriptor().Fields().Get(0), protoreflect.ValueOfString(id))
	return answer(message.Interface())
}

func TestEachIdentityCommandAnswers500ForWhatTheContractCannotRepresent(t *testing.T) {
	for _, command := range identityCommands() {
		cases := map[string]reply{
			"an accepted answer without id":                      answeredWithID(t, command, ""),
			"an accepted answer with an id outside the contract": answeredWithID(t, command, "id 1"),
			"a rejection without code":                           answer(command.rejection(&servicev1.Rejection{Message: "refused"})),
		}
		for name, respond := range cases {
			t.Run(command.name+"/"+name, func(t *testing.T) {
				f, fake := command.edge(t, respond, nil)

				rec := command.send(t, f, command.path, command.baseBody(t))

				requireRejection(t, rec, http.StatusInternalServerError, "internal-failure")
				requireHops(t, fake, command.method, 1)
			})
		}
	}
}
