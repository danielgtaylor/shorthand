package shorthand

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type l = []interface{}

var parseExamples = []struct {
	Name            string
	Existing        interface{}
	Input           string
	Error           string
	ForceStringKeys bool
	Go              interface{}
	JSON            string
}{
	{
		Name:  "Value",
		Input: "true",
		JSON:  `[["", true]]`,
	},
	{
		Name:  "Empty array",
		Input: "[]",
		JSON:  `[["", []]]`,
	},
	{
		Name:  "Empty object",
		Input: "{}",
		JSON:  `[["", {}]]`,
	},
	{
		Name:  "UTF-8 characters",
		Input: "ä",
		JSON:  `[["", "ä"]]`,
	},
	{
		Name:  "Escape property unquoted",
		Input: `a\:\{b: c`,
		JSON:  `[["a\\:\\{b", "c"]]`,
	},
	{
		Name:  "Coercion",
		Input: "{n: null, b: true, i: 1, f: 1.0, fe: 1e+4, dt: 2020-01-01T12:00:00Z, s: hello, b: %wg==}",
		Go: l{
			l{"n"},
			l{"b", true},
			l{"i", 1},
			l{"f", 1.0},
			l{"fe", 10000.0},
			l{"dt", dt},
			l{"s", "hello"},
			l{"b", []byte{0xc2}},
		},
		JSON: `[["n"], ["b", true], ["i", 1], ["f", 1.0], ["fe", 10000], ["dt", "2020-01-01T12:00:00Z"], ["s", "hello"], ["b", "wg=="]]`,
	},
	{
		Name:  "Quoted Coerceable Keys",
		Input: `{"null": 0, "true": 1, "false": 2, "2020-01-01T12:00:00Z": 3, "4": 5}`,
		JSON:  `[["\"null\"", 0], ["\"true\"", 1], ["\"false\"", 2], ["\"2020-01-01T12\\:00\\:00Z\"", 3], ["\"4\"", 5]]`,
	},
	{
		Name:  "Guess object",
		Input: `a: 1`,
		JSON:  `[["a", 1]]`,
	},
	{
		Name:  "Nesting",
		Input: `{a: [[{b: [[1], [{c: [2]}]]}]]}`,
		JSON:  `[["a[0][0].b[0][0]", 1], ["a[0][0].b[1][0].c[0]", 2]]`,
	},
	{
		Name: "Multiline",
		Input: `{
			a: 1
			b{
				c: 2
			}
		}`,
		JSON: `[["a", 1], ["b.c", 2]]`,
	},
	{
		Name: "Spacing weirdness",
		Input: ` {
			a :1

b	{
				c: string  value  	}} `,
		JSON: `[["a", 1], ["b.c", "string  value"]]`,
	},
	{
		Name:  "File include JSON",
		Input: `a: @testdata/hello.json`,
		JSON:  `[["a", {"hello": "world"}]]`,
	},
	{
		Name:  "File include CBOR",
		Input: `a: @testdata/hello.cbor`,
		Go: l{
			l{"a", map[any]any{
				"hello": "world",
				"ints": map[any]any{
					uint64(1): "hello",
					uint64(2): true,
					uint64(3): 4.5,
				},
			}},
		},
	},
	{
		Name:            "File include CBOR string keys",
		Input:           `a: @testdata/hello.cbor`,
		ForceStringKeys: true,
		Go: l{
			l{"a", map[string]any{
				"hello": "world",
				"ints": map[string]any{
					"1": "hello",
					"2": true,
					"3": 4.5,
				},
			}},
		},
	},
	{
		Name:  "File include unstructured text",
		Input: `a: @testdata/hello.txt`,
		Go: l{
			l{"a", "hello\n"},
		},
	},
	{
		Name:  "File include unstructured binary",
		Input: `a: @testdata/binary`,
		Go: l{
			l{"a", []byte{0xc2}},
		},
	},
	{
		Name:  "Unclosed quoted string",
		Input: `"hello`,
		Error: "Expected quote",
	},
	{
		Name:  "Unclosed index EOF",
		Input: `{a[1`,
		Error: "Expected ']'",
	},
	{
		Name:  "Unclosed index other char",
		Input: `{a[1b: 1}`,
		Error: "Expected ']'",
	},
	{
		Name:  "Invalid filename",
		Input: `a: @invalid`,
		Error: "Unable to read file",
	},
}

func TestParser(t *testing.T) {
	for _, example := range parseExamples {
		t.Run(example.Name, func(t *testing.T) {
			t.Logf("Input: %s", example.Input)
			d := NewDocument(
				ParseOptions{
					ForceStringKeys:       example.ForceStringKeys,
					EnableFileInput:       true,
					EnableObjectDetection: true,
					DebugLogger: func(format string, a ...interface{}) {
						t.Logf(format, a...)
					},
				},
			)
			err := d.Parse(example.Input)
			result := d.Marshal()

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

func FuzzParser(f *testing.F) {
	f.Add("{")
	f.Add("}")
	f.Add("[")
	f.Add("]")
	f.Add("null")
	f.Add("true")
	f.Add("0")
	f.Add(`"hello"`)
	f.Add(`"\u0020"`)
	f.Fuzz(func(t *testing.T, s string) {
		d := NewDocument(
			ParseOptions{
				EnableFileInput: true,
				DebugLogger: func(format string, a ...interface{}) {
					t.Logf(format, a...)
				},
			},
		)
		d.Parse(s)
	})
}
