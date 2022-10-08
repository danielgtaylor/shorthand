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
		Name:  "Nested fields",
		Input: `{"f1": {"f2": {"f3": true}}}`,
		Query: `f1.f2.f3`,
		Go:    true,
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
		Name:  "Array item fields",
		Input: `{"items": [{"f1": {"f2": 1}}, {"f1": {"f2": 2}}, {"other": 3}]}`,
		Query: `items.f1.f2`,
		Go:    []any{1.0, 2.0, nil},
	},
	{
		Name:  "Array item fields empty index",
		Input: `{"items": [{"f1": {"f2": 1}}, {"f1": {"f2": 2}}, {"other": 3}]}`,
		Query: `items[].f1.f2`,
		Go:    []any{1.0, 2.0, nil},
	},
	{
		Name:  "Array item scalar filtering",
		Input: `{"items": ["a", "b", "c"]}`,
		Query: `items[@ startsWith a]`,
		Go:    []any{"a"},
	},
	{
		Name:  "Array item filtering",
		Input: `{"items": [{"f1": {"f2": 1}}, {"f1": {"f2": 2}}, {"other": 3}]}`,
		Query: `items[f1 and f1.f2 > 1].f1.f2`,
		Go:    []any{2.0},
	},
	{
		Name:  "Array filtering first match",
		Input: `{"items": ["a", "b", "c"]}`,
		Query: `items[@ startsWith a]|[0]`,
		Go:    "a",
	},
	{
		Name:  "Field selection",
		Input: `{"link": {"id": 1, "verified": true, "tags": ["a", "b"]}}`,
		Query: `link.{id, tags}`,
		Go:    map[string]any{"id": 1.0, "tags": []any{"a", "b"}},
	},
	{
		Name:  "Field expression",
		Input: `{"foo": "bar", "link": {"id": 1, "verified": true, "tags": ["a", "b"]}}`,
		Query: `{foo, id: link.id, tags: link.tags[@ startsWith a]}`,
		Go:    map[string]any{"foo": "bar", "id": 1.0, "tags": []any{"a"}},
	},
	{
		Name:  "Field expression with pipe",
		Input: `{"foo": "bar", "link": {"id": 1, "verified": true, "tags": ["a", "b"]}}`,
		Query: `{foo, tags: link.tags[@ startsWith a]|[0], id: link.id}`,
		Go:    map[string]any{"foo": "bar", "id": 1.0, "tags": "a"},
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

func FuzzGet(f *testing.F) {
	f.Fuzz(func(t *testing.T, s string) {
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
		GetPath(s, data, GetOptions{DebugLogger: t.Logf})
	})
}
