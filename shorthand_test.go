package shorthand

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var getInputExamples = []struct {
	Name   string
	Mode   fs.FileMode
	File   io.Reader
	Input  string
	JSON   string
	Output []byte
}{
	{
		Name:  "No file",
		Mode:  fs.ModeCharDevice,
		Input: "foo[]: 2, bar.another: false, existing: null, existing[]: 1",
		JSON: `{
			"foo": [2],
			"bar": {
				"another": false
			},
			"existing": [1]
		}`,
	},
	{
		Name:   "Raw file",
		File:   strings.NewReader("a text file"),
		Output: []byte("a text file"),
	},
	{
		Name:   "Structured file no args",
		File:   strings.NewReader(`{"foo":"bar"}`),
		Output: []byte(`{"foo":"bar"}`),
	},
	{
		Name: "JSON edit",
		File: strings.NewReader(`{
			"foo": [1],
			"bar": {
				"baz": true
			},
			"existing": [1, 2, 3]
		}`),
		Input: "foo[]: 2, bar.another: false, existing: null, existing[]: 1",
		JSON: `{
			"foo": [1, 2],
			"bar": {
				"another": false,
				"baz": true
			},
			"existing": [1]
		}`,
	},
}

func TestGetInput(t *testing.T) {
	for _, example := range getInputExamples {
		t.Run(example.Name, func(t *testing.T) {
			input := []string{}
			if example.Input != "" {
				input = append(input, example.Input)
			}
			result, isStruct, err := getInput(example.Mode, example.File, input, ParseOptions{
				EnableObjectDetection: true,
			})
			msg := ""
			if e, ok := err.(Error); ok {
				msg = e.Pretty()
			}
			require.NoError(t, err, msg)

			if example.JSON != "" {
				if !isStruct {
					t.Fatal("input not recognized as structured data")
				}
				j, _ := json.Marshal(result)
				assert.JSONEq(t, example.JSON, string(j))
			}

			if example.Output != nil {
				assert.Equal(t, example.Output, result)
			}
		})
	}
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
		Name:   "Empty map",
		Input:  map[string]any{},
		Output: "{}",
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
	{
		Name: "Quoted reserved key",
		Input: map[string]any{
			"a.b": 1,
		},
		Output: `"a.b": 1`,
	},
	{
		Name: "Quoted reserved value",
		Input: map[string]any{
			"v": "a,b",
		},
		Output: `v: "a,b"`,
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

func TestMarshalRoundTripReservedCharacters(t *testing.T) {
	input := map[string]any{
		"a.b":       1,
		"a,b":       "x]y",
		"prefix":    "@file",
		"binary":    "%wg==",
		"comment":   "// hello",
		"space":     "  keep  ",
		"undefined": "undefined",
		"midslash":  "foo//bar",
	}

	marshalled := MarshalCLI(input)
	result, err := Unmarshal(marshalled, ParseOptions{
		EnableObjectDetection: true,
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, input, result)
}

func TestMarshalCLIEmptyMap(t *testing.T) {
	out := MarshalCLI(map[string]any{})
	assert.Equal(t, "{}", out)
	result, err := Unmarshal(out, ParseOptions{}, nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{}, result)
}

func TestQuoteStringFallbackOnMarshalError(t *testing.T) {
	prev := marshalString
	marshalString = func(any) ([]byte, error) {
		return nil, errors.New("boom")
	}
	t.Cleanup(func() {
		marshalString = prev
	})

	assert.Equal(t, "\"bad\ufffd\"", quoteString(string([]byte{'b', 'a', 'd', 0xff})))
}

func TestMarshalCLINonStringMapKeysStayUnquoted(t *testing.T) {
	out := MarshalCLI(map[any]any{
		1:   "one",
		"2": "two",
	})
	assert.Equal(t, `1: one, "2": two`, out)
}

func TestUnmarshalCommentDisambiguation(t *testing.T) {
	commentDT, err := time.Parse(time.RFC3339, "2025-04-03T15:19:22Z")
	require.NoError(t, err)

	tests := []struct {
		name  string
		input string
		want  map[string]any
		err   string
	}{
		{
			name:  "URL without whitespace",
			input: "url: http://test.de:4242",
			want: map[string]any{
				"url": "http://test.de:4242",
			},
		},
		{
			name:  "String with mid-slash",
			input: "value: foo//bar",
			want: map[string]any{
				"value": "foo//bar",
			},
		},
		{
			name:  "Integer with comment",
			input: "a: 1//foo",
			want: map[string]any{
				"a": 1,
			},
		},
		{
			name:  "Boolean with comment",
			input: "a: true//foo",
			want: map[string]any{
				"a": true,
			},
		},
		{
			name:  "String comment requires whitespace",
			input: "a: foo //bar",
			want: map[string]any{
				"a": "foo",
			},
		},
		{
			name:  "URL with trailing comment",
			input: "url: http://test.de:4242 // prod",
			want: map[string]any{
				"url": "http://test.de:4242",
			},
		},
		{
			name:  "Leading comment before value",
			input: "a: //foo\n 1",
			want: map[string]any{
				"a": 1,
			},
		},
		{
			name:  "Array integer with comment",
			input: "[1//foo\n, 2]",
			want: map[string]any{
				"": []any{1, 2},
			},
		},
		{
			name:  "Array string with mid-slash",
			input: "[foo//bar, 2]",
			want: map[string]any{
				"": []any{"foo//bar", 2},
			},
		},
		{
			name:  "Array URLs",
			input: "[http://a, https://b/x//y]",
			want: map[string]any{
				"": []any{"http://a", "https://b/x//y"},
			},
		},
		{
			name:  "Object continues after numeric comment",
			input: "a: 1//foo\nb: 2",
			want: map[string]any{
				"a": 1,
				"b": 2,
			},
		},
		{
			name:  "Object continues after string with mid-slash",
			input: "a: foo//bar, b: 2",
			want: map[string]any{
				"a": "foo//bar",
				"b": 2,
			},
		},
		{
			name:  "Null with comment",
			input: "a: null//foo",
			want: map[string]any{
				"a": nil,
			},
		},
		{
			name:  "Float with comment",
			input: "a: 1.25//foo",
			want: map[string]any{
				"a": 1.25,
			},
		},
		{
			name:  "Exponent with comment",
			input: "a: 1e3//foo",
			want: map[string]any{
				"a": 1000.0,
			},
		},
		{
			name:  "Datetime with comment",
			input: "a: 2025-04-03T15:19:22Z//foo",
			want: map[string]any{
				"a": commentDT,
			},
		},
		{
			name:  "Incomplete exponent stays string",
			input: "a: 1e//foo",
			want: map[string]any{
				"a": "1e//foo",
			},
		},
		{
			name:  "Bare plus stays string",
			input: "a: +//foo",
			want: map[string]any{
				"a": "+//foo",
			},
		},
		{
			name:  "Bare dot stays string",
			input: "a: .//foo",
			want: map[string]any{
				"a": ".//foo",
			},
		},
		{
			name:  "File input with trailing comment",
			input: "a: @testdata/hello.txt//foo",
			want: map[string]any{
				"a": "hello\n",
			},
		},
		{
			name:  "Base64 input with trailing comment",
			input: "a: %wg==//foo",
			want: map[string]any{
				"a": []byte{0xc2},
			},
		},
		{
			name:  "Invalid base64 before comment errors",
			input: "a: %notbase64//foo",
			err:   "Unable to Base64 decode",
		},
		{
			name:  "Top level comment before array",
			input: "//foo\n[1, 2]",
			want: map[string]any{
				"": []any{1, 2},
			},
		},
		{
			name:  "Trailing comment after object",
			input: "{a: 1} //foo",
			want: map[string]any{
				"a": 1,
			},
		},
		{
			name:  "Trailing comment after array",
			input: "[1, 2] //foo",
			want: map[string]any{
				"": []any{1, 2},
			},
		},
		{
			name:  "Multiple consecutive comments before value",
			input: "a: //one\n //two\n 1",
			want: map[string]any{
				"a": 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Unmarshal(tt.input, ParseOptions{
				EnableObjectDetection: true,
				EnableFileInput:       true,
			}, nil)
			if tt.err != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.err)
				return
			}

			require.NoError(t, err)
			if root, ok := tt.want[""]; ok && len(tt.want) == 1 {
				assert.Equal(t, root, result)
			} else {
				assert.Equal(t, tt.want, result)
			}
		})
	}
}
