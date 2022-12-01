package shorthand

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var getExamples = []struct {
	Name  string
	Input interface{}
	Query string
	Error string
	Go    interface{}
	JSON  string
}{
	{
		Name:  "Field",
		Input: `{"field": "value"}`,
		Query: "field",
		Go:    "value",
	},
	{
		Name:  "Field escape",
		Input: `{"a": "value"}`,
		Query: `\u0061`,
		Go:    "value",
	},
	{
		Name: "Field non string",
		Input: map[any]any{
			1: true,
		},
		Query: "1",
		Go:    true,
	},
	{
		Name:  "Nested fields",
		Input: `{"f1": {"f2": {"f3": true}}}`,
		Query: `f1.f2.f3`,
		Go:    true,
	},
	{
		Name:  "Nested fields with arrays",
		Input: `{"f1": [{"f2": {"f3": true}}, {"missing": true}]}`,
		Query: `f1.f2.f3`,
		Go:    []any{true},
	},
	{
		Name:  "Nested fields empty array",
		Input: `{"f1": []}`,
		Query: `f1.f2.f3`,
		Go:    nil,
	},
	{
		Name:  "Wildcard fields",
		Input: `{"f1": {"unknown1": {"id": 1}, "unknown2": {"id": 2}}}`,
		Query: `f1.*.id`,
		Go:    []any{1.0, 2.0},
	},
	{
		Name: "Wildcard field non string",
		Input: map[any]any{
			"f1": map[any]any{
				"unknown1": map[any]any{
					"id": 1.0,
				},
				"unknown2": map[any]any{
					"id": 2.0,
				},
			},
		},
		Query: `f1.*.id`,
		Go:    []any{1.0, 2.0},
	},
	{
		Name:  "Recursive fields",
		Input: `{"a": [{"id": 1}, {"b": {"id": 2}}], "c": {"d": {"id": 3}}}`,
		Query: `..id`,
		Go:    []any{1.0, 2.0, 3.0},
	},
	{
		Name: "Recursive fields any map",
		Input: map[any]any{
			"a": []any{
				map[any]any{
					"id": 1,
				},
				map[any]any{
					"id": 2,
				},
			},
		},
		Query: `..id`,
		Go:    []any{1, 2},
	},
	{
		Name:  "Array index",
		Input: `{"field": [1, 2, 3]}`,
		Query: `field[0]`,
		Go:    1.0,
	},
	{
		Name:  "Array index out of bounds",
		Input: `{"field": [1, 2, 3]}`,
		Query: `field[5]`,
		Go:    nil,
	},
	{
		Name:  "Array index nested",
		Input: `{"field": [null, [[1]]]}`,
		Query: `field[1][0][0]`,
		Go:    1.0,
	},
	{
		Name:  "Array slice",
		Input: `{"field": [0, 1, 2]}`,
		Query: `field[0:1]`,
		Go:    []any{0.0, 1.0},
	},
	{
		Name:  "Array slice optional start",
		Input: `{"field": [0, 1, 2]}`,
		Query: `field[:1]`,
		Go:    []any{0.0, 1.0},
	},
	{
		Name:  "Array slice optional end",
		Input: `{"field": [0, 1, 2]}`,
		Query: `field[1:]`,
		Go:    []any{1.0, 2.0},
	},
	{
		Name:  "Index string",
		Input: `{"field": "hello"}`,
		Query: `field[1]`,
		Go:    "e",
	},
	{
		Name:  "Slice string",
		Input: `{"field": "hello"}`,
		Query: `field[1:]`,
		Go:    "ello",
	},
	{
		Name:  "Truncate string",
		Input: `{"field": "hello"}`,
		Query: `field[:30]`,
		Go:    "hello",
	},
	{
		Name:  "Index bytes",
		Input: map[string]any{"field": []byte("hello")},
		Query: `field[1]`,
		Go:    uint8('e'),
	},
	{
		Name:  "Slice bytes",
		Input: map[string]any{"field": []byte("hello")},
		Query: `field[1:]`,
		Go:    []byte("ello"),
	},
	{
		Name:  "Array item fields",
		Input: `{"items": [{"f1": {"f2": 1}}, {"f1": {"f2": 2}}, {"other": 3}]}`,
		Query: `items.f1.f2`,
		Go:    []any{1.0, 2.0},
	},
	{
		Name:  "Array item fields empty index",
		Input: `{"items": [{"f1": {"f2": 1}}, {"f1": {"f2": 2}}, {"other": 3}]}`,
		Query: `items[].f1.f2`,
		Go:    []any{1.0, 2.0},
	},
	{
		Name:  "Array item scalar filtering",
		Input: `{"items": ["a", "b", "c"]}`,
		Query: `items[@ startsWith a]`,
		Go:    []any{"a"},
	},
	{
		Name:  "Array item scalar filtering with ?",
		Input: `{"items": ["a", "b", "c"]}`,
		Query: `items[?@ startsWith a]`,
		Go:    []any{"a"},
	},
	{
		Name:  "Array item filtering",
		Input: `{"items": [{"f1": {"f2": 1}}, {"f1": {"f2": 2}}, {"other": 3}]}`,
		Query: `items[f1 and f1.f2 > 1].f1.f2`,
		Go:    []any{2.0},
	},
	{
		Name:  "Array filtering nested brackets",
		Input: `{"items": [{"id": 1, "tags": ["a", "b"]}]}`,
		Query: `items[tags[0] == "abc"[0]].id`,
		Go:    []any{1.0},
	},
	{
		Name:  "Array filtering first match",
		Input: `{"items": ["a", "b", "c"]}`,
		Query: `items[@ startsWith a]|[0]`,
		Go:    "a",
	},
	{
		Name:  "Array filtering escape",
		Input: `{"items": ["a", "b", "c"]}`,
		Query: `items[@ startsWith \u0061]`,
		Go:    []any{"a"},
	},
	{
		Name:  "Field selection",
		Input: `{"link": {"id": 1, "verified": true, "tags": ["a", "b"]}}`,
		Query: `link.{id, tags}`,
		Go:    map[string]any{"id": 1.0, "tags": []any{"a", "b"}},
	},
	{
		Name:  "Field selection quoted",
		Input: `{"link": {"id": 1, "verified": true, "tags ": ["a", "b"]}}`,
		Query: `link.{"id", t: "tags "}`,
		Go:    map[string]any{"id": 1.0, "t": []any{"a", "b"}},
	},
	{
		Name:  "Field selection escaped",
		Input: `{"link": {"a": true}}`,
		Query: `link.{\u0061}`,
		Go:    map[string]any{"a": true},
	},
	{
		Name: "Field selection map any",
		Input: map[any]any{
			"foo": "bar",
			"baz": true,
		},
		Query: `{foo}`,
		Go:    map[string]any{"foo": "bar"},
	},
	{
		Name:  "Array field selection",
		Input: `{"links": [{"rel": "next", "href": "..."}, {"rel": "prev", "href": "..."}]}`,
		Query: `links.{rel}`,
		Go:    []any{map[string]any{"rel": "next"}, map[string]any{"rel": "prev"}},
	},
	{
		Name:  "Field expression",
		Input: `{"foo": "bar", "link": {"id": 1, "verified": true, "tags": ["a", "b"]}}`,
		Query: `{foo, id: link.id, tags: link.tags[@ startsWith a]}`,
		Go:    map[string]any{"foo": "bar", "id": 1.0, "tags": []any{"a"}},
	},
	{
		Name:  "Field expression nested multi",
		Input: `{"body": [{"id": "a", "created": "2022", "link": "..."}], "headers": {"one": 1, "two": 2}}`,
		Query: `{body: body.{id, created}, one: headers.one}`,
		Go:    map[string]any{"body": []any{map[string]any{"id": "a", "created": "2022"}}, "one": 1.0},
	},
	{
		Name:  "Field expression with pipe",
		Input: `{"foo": "bar", "link": {"id": 1, "verified": true, "tags": ["a", "b"]}}`,
		Query: `{foo, tags: link.tags[@ startsWith a]|[0], id: link.id}`,
		Go:    map[string]any{"foo": "bar", "id": 1.0, "tags": "a"},
	},
	{
		Name:  "Field expression with slices",
		Input: `{"items": [1, 2, 3, 4, 5]}`,
		Query: `{first: items[:2], filtered: items[@ > 1]|[:1], last: items[-1:]}`,
		Go:    map[string]any{"first": []any{1.0, 2.0, 3.0}, "filtered": []any{2.0, 3.0}, "last": []any{5.0}},
	},
	{
		Name:  "Unclosed filter",
		Input: `{}`,
		Query: `foo[`,
		Error: "expected ']'",
	},
	{
		Name:  "Unclosed filter quote",
		Input: `{}`,
		Query: `foo["`,
		Error: "Expected quote but found EOF",
	},
	{
		Name:  "Recursive prop unclosed quote",
		Input: `{"foo": "bar"}`,
		Query: `foo.."`,
		Error: "Expected quote but found EOF",
	},
	{
		Name:  "Filter expr error",
		Input: `{}`,
		Query: `foo[1/0]"`,
		Error: "cannot divide by zero",
	},
	{
		Name:  "Field select non-map",
		Input: `[1, 2, 3]`,
		Query: `{id}`,
		Error: "field selection requires a map",
	},
	{
		Name:  "Field select unclosed",
		Input: `[1, 2, 3]`,
		Query: `{id`,
		Error: "field selection requires a map",
	},
}

func TestGet(t *testing.T) {
	for _, example := range getExamples {
		t.Run(example.Name, func(t *testing.T) {
			t.Logf("Input: %s", example.Input)
			input := example.Input
			if s, ok := input.(string); ok {
				require.NoError(t, json.Unmarshal([]byte(s), &input))
			}
			result, _, err := GetPath(example.Query, input, GetOptions{DebugLogger: t.Logf})

			if example.Error == "" {
				msg := ""
				if err != nil {
					msg = err.Pretty()
				}
				require.NoError(t, err, msg)
			} else {
				require.Error(t, err, "result is %v", result)
				require.Contains(t, err.Error(), example.Error)
			}

			if example.Go != nil {
				assert.Equal(t, example.Go, result)
			}

			if example.JSON != "" {
				result = ConvertMapString(result)
				b, _ := json.Marshal(result)
				assert.JSONEq(t, example.JSON, string(b))
			}
		})
	}
}

var getBenchInput = map[string]any{
	"items": []any{
		0,
		map[string]any{
			"id":    1,
			"name":  "Item 1",
			"desc":  "...",
			"price": 4.99,
			"tags":  []any{"one", "two", "three"},
		},
		map[string]any{
			"id":    2,
			"name":  "Item 2",
			"desc":  "...",
			"price": 1.50,
			"tags":  []any{"four", "five", "six"},
		},
	},
}

// func BenchmarkGetJMESPathSimple(b *testing.B) {
// 	b.ReportAllocs()

// 	query := "items[1].name"

// 	out, err := jmespath.Search(query, getBenchInput)
// 	require.NoError(b, err)
// 	require.Equal(b, "Item 1", out)

// 	for n := 0; n < b.N; n++ {
// 		jmespath.Search(query, getBenchInput)
// 	}
// }

// func BenchmarkGetJMESPath(b *testing.B) {
// 	b.ReportAllocs()

// 	query := "items[-1].{name: name, price: price, f: tags[?starts_with(@, `\"f\"`)]}"

// 	out, err := jmespath.Search(query, getBenchInput)
// 	require.NoError(b, err)
// 	require.Equal(b, map[string]any{
// 		"name":  "Item 2",
// 		"price": 1.50,
// 		"f":     []any{"four", "five"},
// 	}, out)

// 	for n := 0; n < b.N; n++ {
// 		jmespath.Search(query, getBenchInput)
// 	}
// }

// func BenchmarkGetJMESPathFlatten(b *testing.B) {
// 	b.ReportAllocs()

// 	query := "items[].tags|[]"

// 	out, err := jmespath.Search(query, getBenchInput)
// 	require.NoError(b, err)
// 	require.Equal(b, []any{"one", "two", "three", "four", "five", "six"}, out)

// 	for n := 0; n < b.N; n++ {
// 		GetPath(query, getBenchInput, GetOptions{})
// 	}
// }

func BenchmarkGetPathSimple(b *testing.B) {
	b.ReportAllocs()

	query := "items[1].name"

	out, _, err := GetPath(query, getBenchInput, GetOptions{})
	require.NoError(b, err)
	require.Equal(b, "Item 1", out)

	for n := 0; n < b.N; n++ {
		GetPath(query, getBenchInput, GetOptions{})
	}
}

func BenchmarkGetPath(b *testing.B) {
	b.ReportAllocs()

	query := "items[-1].{name, price, f: tags[@ startsWith f]}"

	out, _, err := GetPath(query, getBenchInput, GetOptions{})
	require.NoError(b, err)
	require.Equal(b, map[string]any{
		"name":  "Item 2",
		"price": 1.50,
		"f":     []any{"four", "five"},
	}, out)

	for n := 0; n < b.N; n++ {
		GetPath(query, getBenchInput, GetOptions{})
	}
}

func BenchmarkGetPathFlatten(b *testing.B) {
	b.ReportAllocs()

	query := "items.tags|[]"

	out, _, err := GetPath(query, getBenchInput, GetOptions{})
	require.NoError(b, err)
	require.Equal(b, []any{"one", "two", "three", "four", "five", "six"}, out)

	for n := 0; n < b.N; n++ {
		GetPath(query, getBenchInput, GetOptions{})
	}
}

func FuzzGet(f *testing.F) {
	data := map[string]any{
		"n":  nil,
		"b":  true,
		"i":  123,
		"f":  4.5,
		"s":  "hello",
		"b2": []byte{0, 1, 2},
		"d":  time.Now(),
		"a":  []any{1, 2.5, "foo"},
		"aa": []any{[]any{[]any{1, 2, 3}}},
		"am": []any{map[string]any{"a": []any{1, 2, 3}}},
		"m": map[any]any{
			1: true,
		},
	}

	f.Fuzz(func(t *testing.T, s string) {
		GetPath(s, data, GetOptions{DebugLogger: t.Logf})
	})
}
