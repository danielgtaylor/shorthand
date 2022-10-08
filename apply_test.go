package shorthand

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var dt, _ = time.Parse(time.RFC3339, "2020-01-01T12:00:00Z")

var applyExamples = []struct {
	Name     string
	Existing interface{}
	Input    string
	Error    string
	Go       interface{}
	JSON     string
}{
	{
		Name:  "Value",
		Input: "true",
		JSON:  `true`,
	},
	{
		Name:  "Coercion",
		Input: "{n: null, b: true, i: 1, f: 1.0, dt: 2020-01-01T12:00:00Z, s: hello}",
		Go: map[string]interface{}{
			"n":  nil,
			"b":  true,
			"i":  1,
			"f":  1.0,
			"dt": dt,
			"s":  "hello",
		},
		JSON: `{"n": null, "b": true, "i": 1, "f": 1.0, "dt": "2020-01-01T12:00:00Z", "s": "hello"}`,
	},
	{
		Name:  "Property nested",
		Input: "{foo.bar.baz: hello}",
		JSON:  `{"foo": {"bar": {"baz": "hello"}}}`,
	},
	{
		Name:  "Force strings quote",
		Input: `{n: "null", b: "true", i: "1", f: "1.0", s: "hello"}`,
		Go: map[string]interface{}{
			"n": "null",
			"b": "true",
			"i": "1",
			"f": "1.0",
			"s": "hello",
		},
		JSON: `{"n": "null", "b": "true", "i": "1", "f": "1.0", "s": "hello"}`,
	},
	{
		Name:  "Property new type",
		Input: "{foo: [1, 2], foo: true}",
		JSON:  `{"foo": true}`,
	},
	{
		Name:  "Ignore whitespace",
		Input: "{foo :    hello   ,    bar:world  }",
		JSON:  `{"foo": "hello", "bar": "world"}`,
	},
	{
		Name:  "Allow quoted whitespace",
		Input: `{"foo ": "   hello   ", "   bar":"world  "}`,
		JSON:  `{"foo ": "   hello   ", "   bar": "world  "}`,
	},
	{
		Name:  "Coerce trailing space in object",
		Input: "{foo{a: 1 }}",
		JSON:  `{"foo": {"a": 1}}`,
	},
	{
		Name:  "Escape property",
		Input: "{foo\\.bar: baz}",
		JSON:  `{"foo.bar": "baz"}`,
	},
	{
		Name:  "Escape quoted property",
		Input: `{"foo\"bar": baz}`,
		JSON:  `{"foo\"bar": "baz"}`,
	},
	{
		Name:  "Quoted property special chars",
		Input: `{"foo.bar": baz}`,
		JSON:  `{"foo.bar": "baz"}`,
	},
	{
		Name:  "Array",
		Input: "{foo: [1, 2, 3]}",
		JSON:  `{"foo": [1, 2, 3]}`,
	},
	{
		Name:  "Array indexing",
		Input: "{foo[3]: three, foo[5]: five, foo[0]: true}",
		JSON:  `{"foo": [true, null, null, "three", null, "five"]}`,
	},
	{
		Name:  "Append",
		Input: "{foo[]: 1, foo[]: 2, foo[]: 3}",
		JSON:  `{"foo": [1, 2, 3]}`,
	},
	{
		Name:  "Insert prepend",
		Input: "{foo: [1, 2], foo[^0]: 0}",
		JSON:  `{"foo": [0, 1, 2]}`,
	},
	{
		Name:  "Insert middle",
		Input: "{foo: [1, 2], foo[^1]: 0}",
		JSON:  `{"foo": [1, 0, 2]}`,
	},
	{
		Name:  "Insert after",
		Input: "{foo: [1, 2], foo[^3]: 0}",
		JSON:  `{"foo": [1, 2, null, 0]}`,
	},
	{
		Name:  "Nested array",
		Input: "{foo[][1][]: 1}",
		JSON:  `{"foo": [[null, [1]]]}`,
	},
	{
		Name:  "Complex nested array",
		Input: "{foo[][]: 1, foo[0][0][]: [2, 3], bar[]: true, bar[0]: false}",
		JSON:  `{"foo": [[[[2, 3]]]], "bar": [false]}`,
	},
	{
		Name:  "List of objects",
		Input: "{foo[]{id: 1, count: 1}, foo[]{id: 2, count: 2}}",
		JSON:  `{"foo": [{"id": 1, "count": 1}, {"id": 2, "count": 2}]}`,
	},
	{
		Name:  "JSON input",
		Input: `{"null": null, "bool": true, "num": 1.5, "str": "hello", "arr": ["tag1", "tag2"], "obj": {"id": [1]}}`,
		JSON:  `{"null": null, "bool": true, "num": 1.5, "str": "hello", "arr": ["tag1", "tag2"], "obj": {"id": [1]}}`,
	},
	{
		Name:  "JSON naked escapes",
		Input: `{foo\u000Abar: a\nb, baz\ta: a\nb}`,
		JSON:  `{"foo\nbar": "a\nb", "baz\ta": "a\nb"}`,
	},
	{
		Name:  "JSON string escapes",
		Input: `{"foo\u000Abar": "a\nb", "baz\ta": "a\nb"}`,
		JSON:  `{"foo\nbar": "a\nb", "baz\ta": "a\nb"}`,
	},
	{
		Name:  "Top-level array",
		Input: `[1, 2, "hello"]`,
		JSON:  `[1, 2, "hello"]`,
	},
	{
		Name:  "Non-string keys",
		Input: `{1: a, 2.3: b, bar.baz.4: c}`,
		Go: map[interface{}]interface{}{
			1: "a",
			2: map[interface{}]interface{}{
				3: "b",
			},
			"bar": map[string]interface{}{
				"baz": map[interface{}]interface{}{
					4: "c",
				},
			},
		},
		JSON: `{"1": "a", "2": {"3": "b"}, "bar": {"baz": {"4": "c"}}}`,
	},
	{
		Name:  "Key type reset",
		Input: `{foo: true, 2: false}`,
		Go: map[interface{}]interface{}{
			"foo": true,
			2:     false,
		},
		JSON: `{"foo": true, "2": false}`,
	},
	{
		Name:  "Nested key type reset",
		Input: `{foo.bar: true, foo.2: false, foo.2.baz: hello, foo.2.3: false}`,
		Go: map[string]interface{}{
			"foo": map[interface{}]interface{}{
				"bar": true,
				2: map[interface{}]interface{}{
					"baz": "hello",
					3:     false,
				},
			},
		},
		JSON: `{"foo": {"bar": true, "2": {"baz": "hello", "3": false}}}`,
	},
	{
		Name: "Existing",
		Existing: map[string]interface{}{
			"foo": []interface{}{1, 2},
			"bar": []interface{}{[]interface{}{1}},
			"baz": map[string]interface{}{
				"id": 1,
			},
			"hello": "world",
		},
		Input: `{foo[]: 3, foo[]: 4, bar[0][]: 2, baz.another: test}`,
		JSON: `{
			"foo": [1, 2, 3, 4],
			"bar": [[1, 2]],
			"baz": {
				"id": 1,
				"another": "test"
			},
			"hello": "world"
		}`,
	},
	{
		Name: "Unset property",
		Existing: map[string]interface{}{
			"foo": true,
			"bar": 1,
		},
		Input: "{bar: undefined}",
		JSON:  `{"foo": true}`,
	},
	{
		Name: "Unset array item",
		Existing: map[string]interface{}{
			"foo": []interface{}{1, 2, 3, 4},
		},
		Input: "{foo[1]: undefined}",
		JSON:  `{"foo": [1, 3, 4]}`,
	},
	{
		Name: "Move property",
		Existing: map[string]interface{}{
			"foo": "hello",
		},
		Input: "{bar ^ foo}",
		JSON:  `{"bar": "hello"}`,
	},
	{
		Name: "Swap property",
		Existing: map[string]interface{}{
			"foo": "hello",
			"bar": "world",
		},
		Input: "{bar ^ foo}",
		JSON:  `{"bar": "hello", "foo": "world"}`,
	},
	{
		Name: "Swap index",
		Existing: map[string]interface{}{
			"foo": []interface{}{1, 2, 3},
		},
		Input: "{bar ^ foo[0]}",
		JSON:  `{"bar": 1, "foo": [2, 3]}`,
	},
}

func TestApply(t *testing.T) {
	for _, example := range applyExamples {
		t.Run(example.Name, func(t *testing.T) {
			t.Logf("Input: %s", example.Input)
			d := NewDocument(
				ParseOptions{
					EnableFileInput: true,
					DebugLogger:     t.Logf,
				},
			)
			err := d.Parse(example.Input)
			if example.Error == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err, "result is %v", d.Operations)
				require.Contains(t, err.Error(), example.Error)
			}

			ops := d.Marshal()
			b, _ := json.Marshal(ops)
			t.Log(string(b))

			result, err := d.Apply(example.Existing)
			if example.Error == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err, "result is %v", d.Operations)
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
