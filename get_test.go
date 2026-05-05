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
		Go:    []any{},
	},
	{
		Name:  "Array item null field preserved",
		Input: `{"items": [{"id": 1}, {"id": null}, {"other": 2}]}`,
		Query: `items.id`,
		Go:    []any{1.0, nil},
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
		Name:  "Index string unicode",
		Input: `{"field": "a😈b"}`,
		Query: `field[1]`,
		Go:    "😈",
	},
	{
		Name:  "Slice string unicode",
		Input: `{"field": "a😈b"}`,
		Query: `field[1:]`,
		Go:    "😈b",
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
		Name:  "Root array index still works",
		Input: `["a", "b", "c"]`,
		Query: `[0]`,
		Go:    "a",
	},
	{
		Name:  "Root array slice still works",
		Input: `["a", "b", "c"]`,
		Query: `[:1]`,
		Go:    []any{"a", "b"},
	},
	{
		Name:  "Root array filter still works",
		Input: `["a", "b", "c"]`,
		Query: `[@ startsWith a]`,
		Go:    []any{"a"},
	},
	{
		Name:  "Array literal multiple elements",
		Input: `{"body": {"val1": "one", "val2": "two"}}`,
		Query: `[body.val1, body.val2]`,
		Go:    []any{"one", "two"},
	},
	{
		Name:  "Array literal in field expression",
		Input: `{"body": {"val1": "one", "val2": "two"}}`,
		Query: `{foo: [body.val1, body.val2]}`,
		Go:    map[string]any{"foo": []any{"one", "two"}},
	},
	{
		Name:  "Array literal in field expression with piped element",
		Input: `{"items": ["a", "b"], "other": "z"}`,
		Query: `{foo: [items|[0], other]}`,
		Go:    map[string]any{"foo": []any{"a", "z"}},
	},
	{
		Name:  "Array literal with nested field expression",
		Input: `{"body": {"val1": "one", "val2": "two"}}`,
		Query: `[{foo: body.val1}, body.val2]`,
		Go:    []any{map[string]any{"foo": "one"}, "two"},
	},
	{
		Name:  "Array literal with nested multi-field expression",
		Input: `{"body": {"val1": "one", "val2": "two", "val3": "three"}}`,
		Query: `[{foo: body.val1, bar: body.val2}, body.val3]`,
		Go:    []any{map[string]any{"foo": "one", "bar": "two"}, "three"},
	},
	{
		Name:  "Array literal with quoted comma in piped element",
		Input: `{"items": [{"id": "a,b", "name": "hit"}], "other": "z"}`,
		Query: `[items[id == "a,b"].name|[0], other]`,
		Go:    []any{"hit", "z"},
	},
	{
		Name:  "Array literal missing element preserved",
		Input: `{"body": {"val1": "one", "val2": "two"}}`,
		Query: `[body.val1, body.missing, body.val2]`,
		Go:    []any{"one", nil, "two"},
	},
	{
		Name:  "Root empty brackets flatten",
		Input: `[[1, 2], 3, [[4]]]`,
		Query: `[]`,
		Go:    []any{1.0, 2.0, 3.0, []any{4.0}},
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
		Name:  "Field selection quoted special chars",
		Input: `{"link": {"foo.bar": 1, "other": 2}}`,
		Query: `link.{"foo.bar"}`,
		Go:    map[string]any{"foo.bar": 1.0},
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
		Name:  "Field expression nested multi path",
		Input: `{"body": [{"id": "a", "created": "2022", "link": "..."}], "headers": {"one": 1, "two": 2}}`,
		Query: `{body: body.{id, created: created}, one: headers.one}`,
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
	{
		Name:  "Array literal unclosed",
		Input: `{}`,
		Query: `[`,
		Error: "expected ']' after index or filter",
	},
	{
		Name:  "Array literal unclosed after comma",
		Input: `{}`,
		Query: `[body.a,`,
		Error: "expected ']' after array literal",
	},
	{
		Name:  "Array literal trailing comma",
		Input: `{}`,
		Query: `[body.a,]`,
		Error: "expected array literal element",
	},
	{
		Name:  "Array literal nested in field unclosed",
		Input: `{}`,
		Query: `{foo: [body.a}`,
		Error: "expected ']' after index or filter",
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

// TestGetPathFound verifies the found return value in edge cases.
func TestGetPathFound(t *testing.T) {
	data := map[string]any{
		"a": map[string]any{"id": 1},
		"b": map[string]any{"id": nil},
		"c": []any{
			map[string]any{"id": 10},
			map[string]any{"id": nil},
		},
	}

	// Empty path: returns input with found=false (no path was evaluated).
	_, found, err := GetPath("", data, GetOptions{})
	require.NoError(t, err)
	assert.False(t, found, "empty path should return found=false")

	// Present field: found=true.
	_, found, err = GetPath("a.id", data, GetOptions{})
	require.NoError(t, err)
	assert.True(t, found)

	// Missing field: found=false.
	_, found, err = GetPath("a.missing", data, GetOptions{})
	require.NoError(t, err)
	assert.False(t, found)

	// Recursive descent returns found=true when results exist.
	result, found, err := GetPath("..id", data, GetOptions{})
	require.NoError(t, err)
	assert.True(t, found)
	assert.NotEmpty(t, result)
}

func TestGetPathAdditionalEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		query  string
		want   any
		found  bool
		errMsg string
	}{
		{
			name:  "Escaped unquoted property path",
			input: map[string]any{"a.b": map[string]any{"c[d]": "ok"}},
			query: `a\.b.c\[d\]`,
			want:  "ok",
			found: true,
		},
		{
			name:  "Negative unicode string slice",
			input: map[string]any{"field": "a😈bc"},
			query: `field[-2:]`,
			want:  "bc",
			found: true,
		},
		{
			name:  "Negative byte slice",
			input: map[string]any{"field": []byte("hello")},
			query: `field[-2:]`,
			want:  []byte("lo"),
			found: true,
		},
		{
			name:  "Flatten non array returns nil",
			input: map[string]any{"field": "hello"},
			query: `field|[]`,
			want:  nil,
			found: true,
		},
		{
			name:  "Single bracket expression is not array literal",
			input: map[string]any{"body": map[string]any{"subject": "docs"}},
			query: `[body.subject]`,
			want:  nil,
			found: true,
		},
		{
			name:  "Single bracket expression in field is not array literal",
			input: map[string]any{"body": map[string]any{"subject": "docs"}},
			query: `{foo: [body.subject]}`,
			want:  map[string]any{"foo": nil},
			found: true,
		},
		{
			name: "Root filter function comma is not array literal",
			input: []any{
				map[string]any{
					"id": 1,
					"keep": func(a, b string) bool {
						return a == "a" && b == "b"
					},
				},
			},
			query: `[keep("a", "b")].id`,
			want:  []any{1},
			found: true,
		},
		{
			name:  "Filter with nested path on scalar input is empty",
			input: map[string]any{"items": []any{"a", "b"}},
			query: `items[@.missing]`,
			want:  []any{},
			found: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found, err := GetPath(tt.query, tt.input, GetOptions{})
			if tt.errMsg != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.found, found)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetFieldSelectionUnescapesOutputKeys(t *testing.T) {
	input := map[string]any{
		"link": map[string]any{
			"foo.bar": 1,
			"a[b]":    2,
		},
	}

	result, found, err := GetPath(`link.{"foo.bar", "a[b]"}`, input, GetOptions{})
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, map[string]any{
		"foo.bar": 1,
		"a[b]":    2,
	}, result)
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
