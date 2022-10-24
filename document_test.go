package shorthand

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func BenchmarkMinJSON(b *testing.B) {
	b.ReportAllocs()
	var v interface{}

	for n := 0; n < b.N; n++ {
		assert.NoError(b, json.Unmarshal([]byte(`{"foo": {"bar": {"id": 1, "tags": ["one", "two"], "cost": 3.14}, "baz": {"id": 2}}}`), &v))
	}
}

func BenchmarkFormattedJSON(b *testing.B) {
	b.ReportAllocs()
	var v interface{}

	large := []byte(`{
		"foo": {
			"bar": {
				"id": 1,
				"tags": ["one", "two"],
				"cost": 3.14
			},
			"baz": {
				"id": 2
			}
		}
	}`)

	for n := 0; n < b.N; n++ {
		assert.NoError(b, json.Unmarshal(large, &v))
	}
}

func BenchmarkYAML(b *testing.B) {
	b.ReportAllocs()
	var v interface{}

	large := []byte(`
    foo:
      bar:
        id: 1
        tags: [one, two]
        cost: 3.14
      baz:
        id: 2
`)

	for n := 0; n < b.N; n++ {
		assert.NoError(b, yaml.Unmarshal(large, &v))
	}
}

func BenchmarkShorthand(b *testing.B) {
	b.ReportAllocs()

	d := NewDocument(ParseOptions{
		ForceStringKeys: true,
	})

	for n := 0; n < b.N; n++ {
		d.Operations = d.Operations[:0]
		d.Parse(`{foo{bar{id: 1, tags: [one, two], cost: 3.14}, baz{id: 2}}}`)
		d.Apply(nil)
	}
}

func BenchmarkPretty(b *testing.B) {
	b.ReportAllocs()

	d := NewDocument(ParseOptions{
		ForceStringKeys: true,
	})

	for n := 0; n < b.N; n++ {
		d.Operations = d.Operations[:0]
		d.Parse(`{
			foo{
				bar{
					id: 1
					tags: [one, two]
					cost: 3.14
				}
				baz{
					id: 2
				}
			}
		}`)
		d.Apply(nil)
	}
}

func BenchmarkParse(b *testing.B) {
	b.ReportAllocs()

	d := NewDocument(ParseOptions{
		ForceStringKeys: true,
	})

	for n := 0; n < b.N; n++ {
		d.Operations = d.Operations[:0]
		d.Parse(`{foo{bar{id: 1, tags: [one, two], cost: 3.14}, baz{id: 2}}}`)
	}
}

func BenchmarkApply(b *testing.B) {
	b.ReportAllocs()
	d := NewDocument(ParseOptions{
		ForceStringKeys: true,
	})
	d.Parse(`{foo{bar{id: 1, tags: [one, two], cost: 3.14}, baz{id: 2}}}`)

	for n := 0; n < b.N; n++ {
		d.Apply(nil)
	}
}
