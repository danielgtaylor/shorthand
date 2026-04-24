package shorthand

import (
	"encoding/json"
	"fmt"
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
		Name:  "Field select unclosed on map",
		Input: `{"id": 1}`,
		Query: `{id`,
		Error: "expected '}' to close field selection",
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

func TestGetPathRealWorldStyleQueries(t *testing.T) {
	t.Run("Restish style collection query", func(t *testing.T) {
		query := "items[status == ready].{id, title, self: _links.self.href, author: content.author.login, tags: content.tags, latency: content.stats.response_ms}"

		inputA, expectedA := makeRestishQueryFixture(0)
		got, found, err := GetPath(query, inputA, GetOptions{})
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, expectedA, got)

		// Reuse the same compiled query against a different input to ensure no
		// execution state leaks through the cache.
		inputB, expectedB := makeRestishQueryFixture(1)
		got, found, err = GetPath(query, inputB, GetOptions{})
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, expectedB, got)
	})

	t.Run("AWS style volume query", func(t *testing.T) {
		query := "Volumes[VolumeType == gp3].{id: VolumeId, az: AvailabilityZone, attachedInstance: Attachments.InstanceId|[0], name: Tags[Key == Name].Value|[0]}"

		inputA, expectedA := makeAWSVolumeQueryFixture(0)
		got, found, err := GetPath(query, inputA, GetOptions{})
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, expectedA, got)

		inputB, expectedB := makeAWSVolumeQueryFixture(1)
		got, found, err = GetPath(query, inputB, GetOptions{})
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, expectedB, got)
	})
}

func TestGetPathCachedMalformedQueries(t *testing.T) {
	t.Run("unclosed field selection keeps input-sensitive error precedence", func(t *testing.T) {
		query := `{id`

		_, _, err := GetPath(query, []any{1.0, 2.0}, GetOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "field selection requires a map")

		_, _, err = GetPath(query, map[string]any{"id": 1.0}, GetOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected '}' to close field selection")
	})

	t.Run("invalid filter remains invalid across reuse", func(t *testing.T) {
		query := `items[1/0].id`

		inputA := map[string]any{
			"items": []any{map[string]any{"id": 1.0}},
		}
		_, _, err := GetPath(query, inputA, GetOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot divide by zero")

		inputB := map[string]any{
			"items": []any{map[string]any{"id": 2.0}},
		}
		_, _, err = GetPath(query, inputB, GetOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot divide by zero")
	})
}

func TestGetPathWildcardOrderingIsStable(t *testing.T) {
	input := map[string]any{
		"items": map[string]any{
			"charlie": map[string]any{"id": 3.0},
			"alpha":   map[string]any{"id": 1.0},
			"bravo":   map[string]any{"id": 2.0},
		},
	}

	got, found, err := GetPath(`items.*.id`, input, GetOptions{})
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, []any{1.0, 2.0, 3.0}, got)
}

func TestGetPathRecursiveDescentMixedInput(t *testing.T) {
	input := map[string]any{
		"id": "root",
		"items": []any{
			map[string]any{
				"id": "item-1",
				"child": map[string]any{
					"id": "child-1",
				},
			},
			map[string]any{
				"child": map[string]any{
					"id": "child-2",
				},
			},
		},
		"meta": map[string]any{
			"nested": []any{
				map[string]any{"id": "nested-1"},
				map[string]any{"other": true},
				map[string]any{"id": nil},
			},
		},
	}

	got, found, err := GetPath(`..id`, input, GetOptions{})
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, []any{"root", "child-1", "item-1", "child-2", "nested-1", nil}, got)
}

func makeRestishQueryFixture(shift int) (map[string]any, []any) {
	items := make([]any, 0, 12)
	expected := make([]any, 0, 4)

	for i := 0; i < 12; i++ {
		statuses := []string{"pending", "ready", "retrying"}
		status := statuses[(i+shift)%len(statuses)]

		item := map[string]any{
			"id":     fmt.Sprintf("req-%03d", i+shift*100),
			"status": status,
			"title":  fmt.Sprintf("Request %03d", i+shift*100),
			"_links": map[string]any{
				"self": map[string]any{
					"href": fmt.Sprintf("https://api.example.test/requests/%03d", i+shift*100),
				},
			},
			"content": map[string]any{
				"author": map[string]any{
					"login": fmt.Sprintf("user-%02d", (i+shift)%5),
				},
				"tags": []any{
					fmt.Sprintf("team-%d", (i+shift)%4),
					fmt.Sprintf("tier-%d", ((i+shift)%3)+1),
				},
				"stats": map[string]any{
					"response_ms": float64(100 + ((i + shift) % 10)),
				},
			},
		}

		items = append(items, item)
		if status == "ready" {
			expected = append(expected, map[string]any{
				"id":      item["id"],
				"title":   item["title"],
				"self":    item["_links"].(map[string]any)["self"].(map[string]any)["href"],
				"author":  item["content"].(map[string]any)["author"].(map[string]any)["login"],
				"tags":    item["content"].(map[string]any)["tags"],
				"latency": item["content"].(map[string]any)["stats"].(map[string]any)["response_ms"],
			})
		}
	}

	return map[string]any{"items": items}, expected
}

func makeAWSVolumeQueryFixture(shift int) (map[string]any, []any) {
	volumes := make([]any, 0, 12)
	expected := make([]any, 0, 4)

	for i := 0; i < 12; i++ {
		volumeTypes := []string{"gp3", "io2", "gp2"}
		volumeType := volumeTypes[(i+shift)%len(volumeTypes)]

		attachments := []any{}
		if (i+shift)%3 != 0 {
			attachments = append(attachments, map[string]any{
				"InstanceId": fmt.Sprintf("i-%08d", 30000000+i+shift*100),
				"State":      "attached",
			})
		}

		tags := []any{
			map[string]any{"Key": "Name", "Value": fmt.Sprintf("volume-%03d", i+shift*100)},
			map[string]any{"Key": "Env", "Value": []string{"prod", "stage", "dev"}[(i+shift)%3]},
		}

		volume := map[string]any{
			"VolumeId":         fmt.Sprintf("vol-%08d", 70000000+i+shift*100),
			"VolumeType":       volumeType,
			"AvailabilityZone": fmt.Sprintf("us-west-2%c", 'a'+rune((i+shift)%3)),
			"Attachments":      attachments,
			"Tags":             tags,
		}

		volumes = append(volumes, volume)
		if volumeType == "gp3" {
			attachedInstance := any(nil)
			if len(attachments) > 0 {
				attachedInstance = attachments[0].(map[string]any)["InstanceId"]
			}

			expected = append(expected, map[string]any{
				"id":               volume["VolumeId"],
				"az":               volume["AvailabilityZone"],
				"attachedInstance": attachedInstance,
				"name":             tags[0].(map[string]any)["Value"],
			})
		}
	}

	return map[string]any{"Volumes": volumes}, expected
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
