package api

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestEveryRequestBodyCompilesToAShape(t *testing.T) {
	for _, body := range []any{
		registerOrganizationBody{}, findOrganizationByTaxIDBody{}, addEstablishmentBody{}, registerPersonBody{},
		provisionUserAccountBody{}, requestBankAccountChangeBody{}, registerRepresentationBody{}, inviteUserBody{}, reasonBody{},
	} {
		if _, err := compileShape(reflect.TypeOf(body)); err != nil {
			t.Errorf("compileShape(%T) = %v", body, err)
		}
	}
}

func TestAShapeRefusesWhatItCannotBound(t *testing.T) {
	fields := make([]reflect.StructField, 65)
	for i := range fields {
		fields[i] = reflect.StructField{Name: "F" + strings.Repeat("x", i), Type: reflect.TypeFor[string]()}
	}
	cases := map[string]reflect.Type{
		"65 members":           reflect.StructOf(fields),
		"a list without bound": reflect.TypeFor[struct{ Items []string }](),
		"a list bound by zero": reflect.TypeOf(struct {
			Items []string `maxItems:"0"`
		}{}),
	}
	for name, typ := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := compileShape(typ); err == nil {
				t.Fatalf("compileShape(%s) = nil, want it refused", typ)
			}
		})
	}
}

func TestTheScanStopsAtTheFirstItemPastTheMaxItemsOfItsSchema(t *testing.T) {
	cases := []struct {
		name  string
		typ   reflect.Type
		field string
		item  string
		max   int
	}{
		{"kinds", reflect.TypeFor[*registerOrganizationBody](), "kinds", `"client"`, 4},
		{"externalReferences", reflect.TypeFor[*addEstablishmentBody](), "externalReferences", `{}`, 32},
		{"scopeActTypes", reflect.TypeFor[*registerRepresentationBody](), "scopeActTypes", `"a"`, 32},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			list := func(n int) []byte {
				return []byte(`{"` + tc.field + `":[` + strings.TrimSuffix(strings.Repeat(tc.item+",", n), ",") + `]}`)
			}
			of := mustShapeOf(tc.typ)
			if err := exactShape(list(tc.max), of); err != nil {
				t.Fatalf("%d items = %v, want the bound itself accepted", tc.max, err)
			}
			if err := exactShape(list(tc.max+1), of); !errors.Is(err, errShape) {
				t.Fatalf("%d items = %v, want errShape", tc.max+1, err)
			}
		})
	}
}

func TestAnExcessListCostsTheScanNoAllocation(t *testing.T) {
	raw := []byte(`{"kinds":[` + strings.Repeat(`"client",`, 7000) + `"client"]}`)
	of := mustShapeOf(reflect.TypeFor[*registerOrganizationBody]())

	if allocs := testing.AllocsPerRun(20, func() { _ = exactShape(raw, of) }); allocs != 0 {
		t.Fatalf("exactShape allocates %v times on a list past its bound, want none", allocs)
	}
}

func TestAMemberNameMatchesOnlyOnceUnescaped(t *testing.T) {
	of := mustShapeOf(reflect.TypeFor[*registerOrganizationBody]())
	cases := map[string]error{
		`{"\u006Binds":[]}`:                nil,
		`{"\u004Binds":[]}`:                errShape,
		`{"kinds":[],"\u006Binds":[]}`:     errShape,
		`{"tradeName":"a\"b","kinds":[]}`:  nil,
		`{"tradeName":"a\\","kinds":[]}`:   nil,
		`{"tradeName":"a\\\"","kinds":[]}`: nil,
		`{"tradeName":"a\\"}`:              nil,
		`{"tradeName":"a\"}`:               errShape,
	}
	for raw, want := range cases {
		if err := exactShape([]byte(raw), of); !errors.Is(err, want) {
			t.Errorf("exactShape(%s) = %v, want %v", raw, err, want)
		}
	}
}

func TestASurrogateScanNeverReadsPastTheBody(t *testing.T) {
	cases := map[string]bool{`\`: false, `"a\`: false, `\u`: false, `\ud83d\`: true, `\ud83d\ude0`: true}
	for raw, want := range cases {
		if got := unpairedSurrogate([]byte(raw)); got != want {
			t.Errorf("unpairedSurrogate(%s) = %t, want %t", raw, got, want)
		}
	}
}
