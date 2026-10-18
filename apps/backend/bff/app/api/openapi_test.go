package api_test

import (
	"fmt"
	"net/http"
	"os"
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
	} `yaml:"components"`
}

type response struct {
	Ref     string               `yaml:"$ref"`
	Headers map[string]yaml.Node `yaml:"headers"`
}

type operation struct {
	Parameters []parameter          `yaml:"parameters"`
	Responses  map[string]yaml.Node `yaml:"responses"`
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

// productionRoutes is the inventory of the routes the edge serves; it stays
// empty until the identity routes land (DEVS-112, DEVS-113).
var productionRoutes = map[string]expectation{}

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
