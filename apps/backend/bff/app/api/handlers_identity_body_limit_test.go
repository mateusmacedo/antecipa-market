package api_test

import (
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	kernelhttp "github.com/mateusmacedo/dmpf/libs/backend/go/http"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/api"
)

const kibibyte = 1 << 10

// A code point the schema admits weighs at most 4 bytes of UTF-8, and JSON may
// still write it as a surrogate pair of \u escapes: 12 bytes, 3 times that; an
// ASCII-only character weighs 1 byte and its \u00XX escape 6.
const (
	escapedCodePoint = `\ud83d\ude00`
	escapedASCII     = `\u0041`
)

var exactLength = regexp.MustCompile(`\{(\d+)\}\$$`)

func worstValue(t *testing.T, doc document, declared schema, at string) string {
	t.Helper()
	declared = doc.schema(declared)
	switch {
	case len(declared.Properties) > 0:
		members := make([]string, 0, len(declared.Properties))
		for _, name := range slices.Sorted(maps.Keys(declared.Properties)) {
			members = append(members, strconv.Quote(name)+":"+worstValue(t, doc, declared.Properties[name], at+name+"."))
		}
		return "{" + strings.Join(members, ",") + "}"
	case declared.Type == "array":
		return "[" + strings.Join(worstItems(t, doc, declared, at), ",") + "]"
	case len(declared.Enum) > 0:
		longest := slices.MaxFunc(declared.Enum, func(a, b string) int { return len(a) - len(b) })
		return `"` + escapedEvery(longest) + `"`
	case declared.Type == "integer":
		return "-9223372036854775808"
	case declared.Type == "boolean":
		return "false"
	case declared.Type == "string":
		return `"` + worstText(t, declared, at) + `"`
	}
	t.Fatalf("%s: schema %+v has no worst case under this test", at, declared)
	return ""
}

func worstItems(t *testing.T, doc document, declared schema, at string) []string {
	t.Helper()
	if declared.MaxItems < 1 || declared.Items == nil {
		t.Fatalf("%s: a list without maxItems has no worst case", at)
	}
	item := doc.schema(*declared.Items)
	if declared.UniqueItems && len(item.Enum) > 0 {
		distinct := slices.SortedFunc(slices.Values(item.Enum), func(a, b string) int { return len(b) - len(a) })
		items := make([]string, 0, declared.MaxItems)
		for _, value := range distinct[:min(declared.MaxItems, len(distinct))] {
			items = append(items, `"`+escapedEvery(value)+`"`)
		}
		return items
	}
	return repeated(declared.MaxItems, worstValue(t, doc, item, at+"[]."))
}

func worstText(t *testing.T, declared schema, at string) string {
	t.Helper()
	length := declared.MaxLength
	if match := exactLength.FindStringSubmatch(declared.Pattern); length == 0 && match != nil {
		length, _ = strconv.Atoi(match[1])
	}
	pattern := regexp.MustCompile(declared.Pattern)
	switch {
	case length == 0:
		t.Fatalf("%s: a text without maxLength has no worst case", at)
	case pattern.MatchString("😀"):
		return strings.Repeat(escapedCodePoint, length)
	case !pattern.MatchString(strings.Repeat("A", length)):
		t.Fatalf("%s: pattern %s refuses %d ASCII letters", at, declared.Pattern, length)
	}
	return strings.Repeat(escapedASCII, length)
}

func escapedEvery(text string) string {
	var b strings.Builder
	for _, c := range []byte(text) {
		fmt.Fprintf(&b, `\u%04x`, c)
	}
	return b.String()
}

func worstBodyOf(t *testing.T, route api.Binding) (string, bool) {
	t.Helper()
	declared := contractOperation(t, api.RouteOf(route)).RequestBody
	if declared == nil {
		return "", false
	}
	doc := loadDocument(t, contractFile(api.RouteOf(route)))
	return worstValue(t, doc, declared.Content["application/json"].Schema, ""), true
}

func contractFile(route kernelhttp.Route) string {
	file, _, _ := strings.Cut(route.ContractRef, "#")
	return file
}

func contractOperation(t *testing.T, route kernelhttp.Route) operation {
	t.Helper()
	return loadDocument(t, contractFile(route)).Paths[route.Path][strings.ToLower(route.Method)]
}

func identityKeyedBindings() []api.Binding {
	var keyed []api.Binding
	for _, b := range api.Bindings(routeBudget) {
		if kind := api.KindOf(b); kind == api.RouteCommand || kind == api.RouteLookup {
			keyed = append(keyed, b)
		}
	}
	return keyed
}

func TestEveryKeyedRouteCapsItsBodyAtTheWorstCaseItsSchemaAdmits(t *testing.T) {
	for _, b := range identityKeyedBindings() {
		route := api.RouteOf(b)
		t.Run(route.Name, func(t *testing.T) {
			worst, declared := worstBodyOf(t, b)
			want := int64(0)
			if declared {
				want = min((int64(len(worst))+kibibyte-1)/kibibyte*kibibyte, api.MaxBodyBytes)
			}
			if got := api.BodyLimitOf(b); got != want {
				t.Fatalf("body limit = %d, want %d: the worst body the schema admits has %d bytes, rounded up to a KiB and never past %d (TRP-49)", got, want, len(worst), api.MaxBodyBytes)
			}
		})
	}
}

func TestTheWorstBodyTheSchemaAdmitsFitsTheLimitOfItsRoute(t *testing.T) {
	for _, b := range identityKeyedBindings() {
		route := api.RouteOf(b)
		worst, declared := worstBodyOf(t, b)
		if !declared || int64(len(worst)) > api.MaxBodyBytes {
			continue
		}
		t.Run(route.Name, func(t *testing.T) {
			c := keyedRoute(t, route.Name)
			f, fake := c.edge(t, c.reply, nil)

			rec := c.send(t, f, c.path, worst)

			if rec.Code != c.status {
				t.Fatalf("status = %d for the %d bytes of the worst body, want %d (body %s)", rec.Code, len(worst), c.status, rec.Body.String())
			}
			requireHops(t, fake, c.method, 1)
		})
	}
}

func TestABodyOneByteOverTheLimitOfItsRouteIsRefusedForItsSize(t *testing.T) {
	for _, b := range identityKeyedBindings() {
		route, limit := api.RouteOf(b), api.BodyLimitOf(b)
		for _, declared := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/content-length declared %t", route.Name, declared), func(t *testing.T) {
				c := keyedRoute(t, route.Name)
				f, fake := c.edge(t, c.reply, nil)
				base := c.baseBody(t)
				body := base + strings.Repeat(" ", int(limit)+1-len(base))
				req := httptest.NewRequest(http.MethodPost, c.path, strings.NewReader(body))
				if !declared {
					req.ContentLength = -1
				}
				for name, value := range map[string]string{"Idempotency-Key": "k-1", "Content-Type": "application/json", "Authorization": requesterCredential} {
					req.Header.Set(name, value)
				}
				rec := httptest.NewRecorder()

				f.handler.ServeHTTP(rec, req)

				requireRejection(t, rec, http.StatusBadRequest, "invalid-request")
				if size := strconv.FormatInt(limit, 10); limit > 0 && !strings.Contains(rec.Body.String(), "exceeds the "+size+" bytes") ||
					limit == 0 && !strings.Contains(rec.Body.String(), "takes no request body") {
					t.Fatalf("body = %s, want the size of %d bytes the route accepts", rec.Body.String(), limit)
				}
				if calls := businessHops(fake); calls != 0 {
					t.Fatalf("business calls = %d, want none", calls)
				}
			})
		}
	}
}

func TestEveryListOfARequestBodyCapsItsItemsAtTheMaxItemsOfItsSchema(t *testing.T) {
	bodies := api.RequestBodies()
	for _, b := range identityKeyedBindings() {
		route := api.RouteOf(b)
		typ := bodies[route.Name]
		declared := contractOperation(t, route).RequestBody
		if typ == nil || declared == nil {
			continue
		}
		doc := loadDocument(t, contractFile(route))
		if problems := maxItemsProblems(doc, doc.schema(declared.Content["application/json"].Schema), typ, route.Name+"."); len(problems) > 0 {
			t.Errorf("%v", problems)
		}
	}
}

func maxItemsProblems(doc document, declared schema, typ reflect.Type, at string) []string {
	var problems []string
	typ = indirect(typ)
	for i := range typ.NumField() {
		field := typ.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		property := doc.schema(declared.Properties[name])
		nested := indirect(field.Type)
		if nested.Kind() == reflect.Slice {
			if tag := field.Tag.Get("maxItems"); tag != strconv.Itoa(property.MaxItems) || property.MaxItems < 1 {
				problems = append(problems, fmt.Sprintf("%s%s: maxItems tag %q, contract %d", at, name, tag, property.MaxItems))
			}
			nested = indirect(nested.Elem())
			if property.Items != nil {
				property = doc.schema(*property.Items)
			}
		}
		if nested.Kind() == reflect.Struct {
			problems = append(problems, maxItemsProblems(doc, property, nested, at+name+".")...)
		}
	}
	return problems
}
