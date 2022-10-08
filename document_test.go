package shorthand

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
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

func BenchmarkLatestFull(b *testing.B) {
	b.ReportAllocs()

	d := NewDocument(ParseOptions{
		ForceStringKeys: true,
	})

	for n := 0; n < b.N; n++ {
		d.Operations = d.Operations[:0]
		d.Parse(`{foo{bar{id: 1, "tags": [one, two], cost: 3.14}, baz{id: 2}}}`)
		d.Apply(nil)
	}
}

func BenchmarkLatestParse(b *testing.B) {
	b.ReportAllocs()

	d := NewDocument(ParseOptions{
		ForceStringKeys: true,
	})

	for n := 0; n < b.N; n++ {
		d.Operations = d.Operations[:0]
		d.Parse(`{foo{bar{id: 1, "tags": [one, two], cost: 3.14}, baz{id: 2}}}`)
	}
}

func BenchmarkLatestApply(b *testing.B) {
	b.ReportAllocs()
	d := NewDocument(ParseOptions{
		ForceStringKeys: true,
	})
	d.Parse(`{foo{bar{id: 1, "tags": [one, two], cost: 3.14}, baz{id: 2}}}`)

	for n := 0; n < b.N; n++ {
		d.Apply(nil)
	}
}
