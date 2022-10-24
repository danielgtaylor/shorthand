package shorthand

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetInput(t *testing.T) {
	file := strings.NewReader(`{
		"foo": [1],
		"bar": {
			"baz": true
		},
		"existing": [1, 2, 3]
	}`)

	result, err := getInput(0, file, []string{"foo[]: 2, bar.another: false, existing: null, existing[]: 1"}, ParseOptions{EnableObjectDetection: true})
	assert.NoError(t, err)

	j, _ := json.Marshal(result)
	assert.JSONEq(t, `{
		"foo": [1, 2],
		"bar": {
			"another": false,
			"baz": true
		},
		"existing": [1]
	}`, string(j))
}

var marshalExamples = []struct {
	Name   string
	Input  any
	Output string
}{
	{
		Name:   "Simple",
		Input:  true,
		Output: "true",
	},
	{
		Name: "Simple object",
		Input: map[string]any{
			"foo": "bar",
		},
		Output: "foo: bar",
	},
	{
		Name: "Multi key",
		Input: map[string]any{
			"foo":   "bar",
			"hello": "world",
			"num":   1,
			"empty": nil,
			"bool":  false,
		},
		Output: "bool: false, empty: null, foo: bar, hello: world, num: 1",
	},
	{
		Name: "Nested simple",
		Input: map[string]any{
			"foo": map[string]any{
				"bar": 1,
			},
		},
		Output: "foo.bar: 1",
	},
	{
		Name: "Nested multi key",
		Input: map[string]any{
			"foo": map[string]any{
				"bar": 1,
				"baz": 2,
			},
		},
		Output: "foo{bar: 1, baz: 2}",
	},
	{
		Name: "List of list of items",
		Input: map[string]any{
			"foo": []any{
				[]any{1, 2, 3},
			},
		},
		Output: "foo: [[1, 2, 3]]",
	},
	{
		Name: "List of objects",
		Input: map[string]interface{}{
			"tags": []interface{}{
				map[string]interface{}{
					"id": "tag1",
					"count": map[string]interface{}{
						"clicks": 15,
						"sales":  3,
					},
				},
				map[string]interface{}{
					"id": "tag2",
					"count": map[string]interface{}{
						"clicks": 7,
						"sales":  4,
					},
				},
			},
		},
		Output: "tags: [{count{clicks: 15, sales: 3}, id: tag1}, {count{clicks: 7, sales: 4}, id: tag2}]",
	},
	{
		Name: "Coerced",
		Input: map[string]any{
			"null": "null",
			"bool": "true",
			"num":  "1234",
			"str":  "hello",
		},
		Output: `bool: "true", "null": "null", num: "1234", str: hello`,
	},
	{
		Name: "File",
		Input: map[string]any{
			"multi": "I am\na multiline\n value.",
			"long":  "I am a really long line of text that should probably get loaded from a file",
		},
		Output: "long: @file, multi: @file",
	},
}

func TestMarshal(t *testing.T) {
	for _, example := range marshalExamples {
		t.Run(example.Name, func(t *testing.T) {
			t.Logf("Input: %s", example.Input)
			out := MarshalCLI(example.Input)
			assert.Equal(t, example.Output, out)
		})
	}
}

func TestMarshalPretty(t *testing.T) {
	result := MarshalPretty(map[string]any{
		"foo": 1,
		"bar": []any{
			2, 3,
		},
		"baz": map[string]any{
			"a": map[any]any{
				"b": map[any]any{
					"c": true,
					"d": false,
				},
			},
		},
	})
	assert.Equal(t, `{
  bar: [
    2
    3
  ]
  baz.a.b{
    c: true
    d: false
  }
  foo: 1
}`, result)
}
