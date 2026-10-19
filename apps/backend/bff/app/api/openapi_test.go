package api_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	kernelhttp "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"gopkg.in/yaml.v3"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/api"
)

type document struct {
	Paths      map[string]map[string]operation `yaml:"paths"`
	Components struct {
		Parameters map[string]parameter `yaml:"parameters"`
		Responses  map[string]response  `yaml:"responses"`
		Schemas    map[string]schema    `yaml:"schemas"`
	} `yaml:"components"`
}

type schema struct {
	Ref         string            `yaml:"$ref"`
	Type        string            `yaml:"type"`
	Properties  map[string]schema `yaml:"properties"`
	Required    []string          `yaml:"required"`
	Items       *schema           `yaml:"items"`
	MaxItems    int               `yaml:"maxItems"`
	UniqueItems bool              `yaml:"uniqueItems"`
	MaxLength   int               `yaml:"maxLength"`
	Pattern     string            `yaml:"pattern"`
	Enum        []string          `yaml:"enum"`
}

type requestBody struct {
	Required bool                 `yaml:"required"`
	Content  map[string]mediaType `yaml:"content"`
}

type mediaType struct {
	Schema schema `yaml:"schema"`
}

type response struct {
	Ref     string               `yaml:"$ref"`
	Headers map[string]yaml.Node `yaml:"headers"`
}

type operation struct {
	Parameters  []parameter          `yaml:"parameters"`
	RequestBody *requestBody         `yaml:"requestBody"`
	Responses   map[string]yaml.Node `yaml:"responses"`
}

type parameter struct {
	Ref      string `yaml:"$ref"`
	Name     string `yaml:"name"`
	In       string `yaml:"in"`
	Required bool   `yaml:"required"`
	Schema   struct {
		Pattern   string `yaml:"pattern"`
		MaxLength int    `yaml:"maxLength"`
	} `yaml:"schema"`
}

func (d document) response(node yaml.Node) (response, error) {
	var r response
	if err := node.Decode(&r); err != nil {
		return response{}, fmt.Errorf("response is not an object: %w", err)
	}
	if name, ok := strings.CutPrefix(r.Ref, "#/components/responses/"); ok {
		return d.Components.Responses[name], nil
	}
	return r, nil
}

func (d document) resolve(p parameter) parameter {
	if name, ok := strings.CutPrefix(p.Ref, "#/components/parameters/"); ok {
		return d.Components.Parameters[name]
	}
	return p
}

func (d document) schema(s schema) schema {
	if name, ok := strings.CutPrefix(s.Ref, "#/components/schemas/"); ok {
		return d.schema(d.Components.Schemas[name])
	}
	return s
}

func loadDocument(t *testing.T, file string) document {
	t.Helper()
	raw, err := os.ReadFile("../../../../../" + file)
	if err != nil {
		t.Fatalf("ReadFile(%s) = %v", file, err)
	}
	var doc document
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%s is not YAML: %v", file, err)
	}
	return doc
}

type expectation struct {
	kind    api.RouteKind
	success string
}

func (e expectation) statuses() []string {
	switch e.kind {
	case api.RouteCommand:
		return []string{e.success, "400", "404", "409", "422", "429", "503", "504", "default"}
	case api.RouteListing:
		return []string{"200", "400", "429", "503", "504", "default"}
	default:
		return []string{"200", "400", "404", "429", "503", "504", "default"}
	}
}

var productionRoutes = map[string]expectation{
	"findOrganization":                     {api.RouteQuery, "200"},
	"findPerson":                           {api.RouteQuery, "200"},
	"findRepresentation":                   {api.RouteQuery, "200"},
	"findBankAccountChange":                {api.RouteQuery, "200"},
	"findEstablishmentsByOrganization":     {api.RouteListing, "200"},
	"findRepresentationsByOrganization":    {api.RouteListing, "200"},
	"findMembershipsByOrganization":        {api.RouteListing, "200"},
	"findBankAccountChangesByOrganization": {api.RouteListing, "200"},
	"findMembershipsByUser":                {api.RouteListing, "200"},
	"findEffectivePermissions":             {api.RouteListing, "200"},
	"requestBankAccountChange":             {api.RouteCommand, "201"},
	"registerOrganization":                 {api.RouteCommand, "201"},
	"addEstablishment":                     {api.RouteCommand, "201"},
	"registerPerson":                       {api.RouteCommand, "201"},
	"provisionUserAccount":                 {api.RouteCommand, "201"},
	"registerRepresentation":               {api.RouteCommand, "201"},
	"inviteUser":                           {api.RouteCommand, "201"},
	"suspendUser":                          {api.RouteCommand, "200"},
	"revokeMembership":                     {api.RouteCommand, "200"},
	"approveBankAccountChange":             {api.RouteCommand, "200"},
	"findOrganizationByTaxId":              {api.RouteLookup, "200"},
}

func contractProblems(t *testing.T, route kernelhttp.Route, want expectation) []string {
	t.Helper()
	file, pointer, found := strings.Cut(route.ContractRef, "#/paths/")
	if !found {
		return []string{fmt.Sprintf("ContractRef %q is not a pointer into paths", route.ContractRef)}
	}
	escaped, method, _ := strings.Cut(pointer, "/")
	path := strings.ReplaceAll(escaped, "~1", "/")
	if path != route.Path || method != strings.ToLower(route.Method) {
		return []string{fmt.Sprintf("ContractRef names %s %s, want %s %s", method, path, route.Method, route.Path)}
	}
	doc := loadDocument(t, file)
	op, declared := doc.Paths[path][method]
	if !declared {
		return []string{fmt.Sprintf("%s does not declare %s %s", file, method, path)}
	}

	var problems []string
	byName := map[string]parameter{}
	for _, p := range op.Parameters {
		resolved := doc.resolve(p)
		byName[resolved.Name] = resolved
	}
	if route.Method == http.MethodPost {
		key, ok := byName[api.IdempotencyHeader]
		if !ok || key.In != "header" || !key.Required {
			problems = append(problems, fmt.Sprintf("%s must be a required header (RST-02), got %+v", api.IdempotencyHeader, key))
		}
		if key.Schema.Pattern != ports.IdempotencyKeyPattern || key.Schema.MaxLength != 128 {
			problems = append(problems, fmt.Sprintf("%s schema = %+v, want pattern %s and maxLength 128 (IDM-02)", api.IdempotencyHeader, key.Schema, ports.IdempotencyKeyPattern))
		}
	}
	if correlation, ok := byName[api.CorrelationHeader]; !ok || correlation.In != "header" || correlation.Required {
		problems = append(problems, fmt.Sprintf("%s must be an optional header, got %+v", api.CorrelationHeader, correlation))
	}

	var got []string
	for code, node := range op.Responses {
		got = append(got, code)
		answer, err := doc.response(node)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		_, marked := answer.Headers[api.ReplayedHeader]
		answered := strings.HasPrefix(code, "2") || code == "422"
		switch {
		case want.kind == api.RouteCommand && answered && !marked:
			problems = append(problems, fmt.Sprintf("command response %s does not declare %s (IDM-08)", code, api.ReplayedHeader))
		case want.kind != api.RouteCommand && marked:
			problems = append(problems, fmt.Sprintf("%s response %s declares %s, which only a command replays", want.kind, code, api.ReplayedHeader))
		}
	}
	slices.Sort(got)
	statuses := want.statuses()
	slices.Sort(statuses)
	if !slices.Equal(got, statuses) {
		problems = append(problems, fmt.Sprintf("responses = %v, want %v for a %s", got, statuses, want.kind))
	}
	return problems
}

func TestTheContractsDeclareEveryRouteTheEdgeServes(t *testing.T) {
	bound := api.Bindings(routeBudget)
	if len(bound) != len(productionRoutes) {
		t.Fatalf("the edge serves %d routes and the inventory names %d, want the same routes in both", len(bound), len(productionRoutes))
	}
	for _, b := range bound {
		route := api.RouteOf(b)
		t.Run(route.Name, func(t *testing.T) {
			want, listed := productionRoutes[route.Name]
			if !listed || want.kind != api.KindOf(b) {
				t.Fatalf("%s is a %s route, the inventory says %+v", route.Name, api.KindOf(b), want)
			}
			if problems := contractProblems(t, route, want); len(problems) > 0 {
				t.Fatalf("%s: %v", route.Name, problems)
			}
		})
	}
}

var syntheticRoutes = []struct {
	binding api.Binding
	want    expectation
}{
	{
		api.NewBinding(api.RouteCommand, syntheticRoute("registerOrganization", http.MethodPost, "/identity/organizations", "identity:register-organization"), nil),
		expectation{api.RouteCommand, "201"},
	},
	{
		api.NewBinding(api.RouteCommand, syntheticRoute("suspendUser", http.MethodPost, "/identity/users/{userId}/suspend", "identity:suspend-user"), nil),
		expectation{api.RouteCommand, "200"},
	},
	{
		api.NewBinding(api.RouteLookup, syntheticRoute("findOrganizationByTaxId", http.MethodPost, "/identity/organization-lookups", "identity:find-organization-by-tax-id"), nil),
		expectation{api.RouteLookup, "200"},
	},
	{
		api.NewBinding(api.RouteQuery, syntheticRoute("findOrganization", http.MethodGet, "/identity/organizations/{organizationId}", "identity:find-organization"), nil),
		expectation{api.RouteQuery, "200"},
	},
	{
		api.NewBinding(api.RouteListing, syntheticRoute("findEstablishmentsByOrganization", http.MethodGet, "/identity/organizations/{organizationId}/establishments", "identity:find-establishments-by-organization"), nil),
		expectation{api.RouteListing, "200"},
	},
}

func TestTheOpenAPIVerifierAcceptsEveryRouteKindAsTheContractDeclaresIt(t *testing.T) {
	for _, synthetic := range syntheticRoutes {
		route := api.RouteOf(synthetic.binding)
		t.Run(string(synthetic.want.kind)+"/"+route.Name, func(t *testing.T) {
			if problems := contractProblems(t, route, synthetic.want); len(problems) > 0 {
				t.Fatalf("%s: %v", route.Name, problems)
			}
		})
	}
}

func TestEveryKeyedIdentityRouteIsAPostUnderItsOwnPermission(t *testing.T) {
	permissions := map[string]ports.Permission{}
	for _, keyed := range append(identityCommands(), organizationLookup()) {
		permissions[keyed.name] = ports.Permission(keyed.permission)
	}
	for _, b := range api.Bindings(routeBudget) {
		route := api.RouteOf(b)
		if kind := api.KindOf(b); kind != api.RouteCommand && kind != api.RouteLookup {
			continue
		}
		t.Run(route.Name, func(t *testing.T) {
			if route.Method != http.MethodPost || route.IdempotencyKey != api.IdempotencyHeader || route.Permission != permissions[route.Name] ||
				route.Requires != kernelhttp.RequireSubjectAndTenant || route.Budget != routeBudget {
				t.Fatalf("route = %+v, want a POST keyed by %s under %q for a subject and a tenant", route, api.IdempotencyHeader, permissions[route.Name])
			}
			if err := route.ValidateEdge(); err != nil {
				t.Fatalf("ValidateEdge() = %v", err)
			}
		})
	}
}

func TestTheOpenAPIVerifierRefusesARouteOfTheWrongKind(t *testing.T) {
	for _, c := range []struct {
		route string
		as    expectation
	}{
		{"findOrganizationByTaxId", expectation{api.RouteCommand, "200"}},
		{"registerOrganization", expectation{api.RouteLookup, "200"}},
		{"findOrganization", expectation{api.RouteListing, "200"}},
		{"findEstablishmentsByOrganization", expectation{api.RouteQuery, "200"}},
		{"registerOrganization", expectation{api.RouteCommand, "200"}},
	} {
		t.Run(c.route+" as "+string(c.as.kind), func(t *testing.T) {
			for _, synthetic := range syntheticRoutes {
				if route := api.RouteOf(synthetic.binding); route.Name == c.route {
					if problems := contractProblems(t, route, c.as); len(problems) == 0 {
						t.Fatalf("%s checked as %+v = no problem, want the verifier to refuse it", c.route, c.as)
					}
					return
				}
			}
			t.Fatalf("no synthetic route %s", c.route)
		})
	}
}

func TestTheEdgeMountsEveryRouteKindTheContractDeclares(t *testing.T) {
	f := newFixture(t, withBindings(func(*probe) []api.Binding {
		bound := make([]api.Binding, len(syntheticRoutes))
		for i, synthetic := range syntheticRoutes {
			bound[i] = synthetic.binding
		}
		return bound
	}))

	rec := f.do(t, http.MethodGet, "/identity/organizations/org-1", nil, "Authorization", "")

	requireRejection(t, rec, http.StatusUnauthorized, "unauthenticated")
}

func indirect(typ reflect.Type) reflect.Type {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}

func jsonFields(typ reflect.Type) map[string]reflect.Type {
	typ = indirect(typ)
	fields := make(map[string]reflect.Type, typ.NumField())
	for i := range typ.NumField() {
		field := typ.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		fields[name] = field.Type
	}
	return fields
}

func shapeProblems(doc document, declared schema, typ reflect.Type, at string) []string {
	fields := jsonFields(typ)
	names := slices.Sorted(maps.Keys(fields))
	if properties := slices.Sorted(maps.Keys(declared.Properties)); !slices.Equal(names, properties) {
		return []string{fmt.Sprintf("%sproperties: bff %v, contract %v", at, names, properties)}
	}
	var problems []string
	for _, name := range names {
		nested, property, label := indirect(fields[name]), doc.schema(declared.Properties[name]), at+name+"."
		if nested.Kind() == reflect.Slice {
			nested, label = indirect(nested.Elem()), at+name+"[]."
			if property.Items != nil {
				property = doc.schema(*property.Items)
			}
		}
		if nested.Kind() == reflect.Struct {
			problems = append(problems, shapeProblems(doc, property, nested, label)...)
		}
	}
	return problems
}

func objectAt(body map[string]any, path []any) map[string]any {
	var node any = body
	for _, step := range path {
		switch step := step.(type) {
		case string:
			node = node.(map[string]any)[step]
		case int:
			node = node.([]any)[step]
		}
	}
	return node.(map[string]any)
}

func (c identityCommand) acceptsWithout(t *testing.T, probe map[string]any, path []any, field string) bool {
	t.Helper()
	body := cloned(t, probe)
	delete(objectAt(body, path), field)
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal = %v", err)
	}
	f, fake := c.edge(t, c.reply, nil)

	rec := c.send(t, f, c.path, string(raw))

	switch rec.Code {
	case c.status:
		return true
	case http.StatusBadRequest:
		requireRejection(t, rec, http.StatusBadRequest, "invalid-request")
		requireHops(t, fake, c.method, 0)
		return false
	default:
		t.Fatalf("without %s: status = %d, want %d or 400 (body %s)", field, rec.Code, c.status, rec.Body.String())
		return false
	}
}

func requiredProblems(t *testing.T, doc document, declared schema, c identityCommand, probe map[string]any, path []any, at string) []string {
	t.Helper()
	object := objectAt(probe, path)
	names := slices.Sorted(maps.Keys(object))
	if properties := slices.Sorted(maps.Keys(declared.Properties)); !slices.Equal(names, properties) {
		return []string{fmt.Sprintf("%sthe probe body names %v, the contract %v", at, names, properties)}
	}
	var problems, required []string
	for _, name := range names {
		if !c.acceptsWithout(t, probe, path, name) {
			required = append(required, name)
		}
		items, listed := object[name].([]any)
		property := doc.schema(declared.Properties[name])
		if !listed || len(items) == 0 || property.Items == nil {
			continue
		}
		if _, nested := items[0].(map[string]any); nested {
			problems = append(problems, requiredProblems(t, doc, doc.schema(*property.Items), c, probe, append(slices.Clone(path), name, 0), at+name+"[].")...)
		}
	}
	if contract := slices.Sorted(slices.Values(declared.Required)); !slices.Equal(required, contract) {
		problems = append(problems, fmt.Sprintf("%srequired: the edge refuses a body without %v, the contract requires %v", at, required, contract))
	}
	return problems
}

func TestTheBodyOfEveryKeyedRouteIsTheRequestBodyOfTheContract(t *testing.T) {
	bodies := api.RequestBodies()
	for _, b := range api.Bindings(routeBudget) {
		if kind := api.KindOf(b); kind != api.RouteCommand && kind != api.RouteLookup {
			continue
		}
		route := api.RouteOf(b)
		t.Run(route.Name, func(t *testing.T) {
			typ, listed := bodies[route.Name]
			if !listed {
				t.Fatalf("api.RequestBodies() does not name %s", route.Name)
			}
			file, _, _ := strings.Cut(route.ContractRef, "#")
			doc := loadDocument(t, file)
			declared := doc.Paths[route.Path][strings.ToLower(route.Method)].RequestBody
			probe := keyedRoute(t, route.Name)
			if typ == nil || declared == nil || probe.body == nil {
				if typ != nil || declared != nil || probe.body != nil {
					t.Fatalf("body in the bff %t, in the contract %t, in the probe %t: want none or one in all three", typ != nil, declared != nil, probe.body != nil)
				}
				return
			}
			media, isJSON := declared.Content["application/json"]
			if !declared.Required || !isJSON || len(declared.Content) != 1 {
				t.Fatalf("requestBody = %+v, want a required application/json body only", declared)
			}
			body := doc.schema(media.Schema)
			if problems := shapeProblems(doc, body, typ, ""); len(problems) > 0 {
				t.Fatalf("%s: %v", typ, problems)
			}
			if problems := requiredProblems(t, doc, body, probe, cloned(t, probe.body), nil, ""); len(problems) > 0 {
				t.Fatalf("%s: %v", typ, problems)
			}
		})
	}
}
