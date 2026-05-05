package benchmarks

import (
	"encoding/json"
	"fmt"
	"testing"

	shorthand "github.com/danielgtaylor/shorthand/v2"
	"github.com/itchyny/gojq"
	"github.com/jmespath/go-jmespath"
)

var benchmarkSink any

type benchmarkCase struct {
	name       string
	input      any
	expected   any
	shorthand  string
	jmespath   string
	jq         string
	sourceNote string
}

type queryRunner struct {
	name string
	run  func(query string, input any) (any, error)
}

var queryRunners = []queryRunner{
	{name: "shorthand", run: runShorthand},
	{name: "jmespath", run: runJMESPath},
	{name: "jq", run: runGoJQ},
}

var benchmarkCases = []benchmarkCase{
	makeSimpleLookupCase(),
	makeComplexProjectionCase(),
	makeRestishCollectionCase(),
	makeAWSVolumesCase(),
}

func TestQueryEnginesMatchExpected(t *testing.T) {
	t.Helper()

	for _, tc := range benchmarkCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			expectedJSON := mustCanonicalJSON(tc.expected)

			for _, runner := range queryRunners {
				runner := runner
				t.Run(runner.name, func(t *testing.T) {
					result, err := runner.run(queryForRunner(tc, runner.name), tc.input)
					if err != nil {
						t.Fatalf("%s query failed: %v", runner.name, err)
					}

					gotJSON := mustCanonicalJSON(result)
					if gotJSON != expectedJSON {
						t.Fatalf("%s result mismatch\nexpected: %s\ngot:      %s", runner.name, expectedJSON, gotJSON)
					}
				})
			}
		})
	}
}

func BenchmarkQueryEngines(b *testing.B) {
	for _, tc := range benchmarkCases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			expectedJSON := mustCanonicalJSON(tc.expected)

			for _, runner := range queryRunners {
				runner := runner
				b.Run(runner.name, func(b *testing.B) {
					query := queryForRunner(tc, runner.name)
					result, err := runner.run(query, tc.input)
					if err != nil {
						b.Fatalf("%s query failed: %v", runner.name, err)
					}

					gotJSON := mustCanonicalJSON(result)
					if gotJSON != expectedJSON {
						b.Fatalf("%s result mismatch\nexpected: %s\ngot:      %s", runner.name, expectedJSON, gotJSON)
					}

					b.ReportAllocs()
					b.ResetTimer()

					for i := 0; i < b.N; i++ {
						result, err := runner.run(query, tc.input)
						if err != nil {
							b.Fatalf("%s query failed: %v", runner.name, err)
						}
						benchmarkSink = result
					}
				})
			}
		})
	}
}

func runShorthand(query string, input any) (any, error) {
	result, found, err := shorthand.GetPath(query, input, shorthand.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("no shorthand result for query %q", query)
	}
	return result, nil
}

func runJMESPath(query string, input any) (any, error) {
	return jmespath.Search(query, input)
}

func runGoJQ(query string, input any) (any, error) {
	parsed, err := gojq.Parse(query)
	if err != nil {
		return nil, err
	}

	code, err := gojq.Compile(parsed)
	if err != nil {
		return nil, err
	}

	iter := code.Run(input)
	results := make([]any, 0, 1)
	for {
		value, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := value.(error); ok {
			return nil, err
		}
		results = append(results, value)
	}

	switch len(results) {
	case 0:
		return nil, nil
	case 1:
		return results[0], nil
	default:
		return results, nil
	}
}

func queryForRunner(tc benchmarkCase, runner string) string {
	switch runner {
	case "shorthand":
		return tc.shorthand
	case "jmespath":
		return tc.jmespath
	case "jq":
		return tc.jq
	default:
		panic("unknown runner " + runner)
	}
}

func mustCanonicalJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func makeSimpleLookupCase() benchmarkCase {
	input := map[string]any{
		"items": []any{
			float64(0),
			map[string]any{
				"id":    float64(1),
				"name":  "Item 1",
				"desc":  "Base item for simple projections",
				"price": 4.99,
				"tags":  []any{"one", "two", "three"},
			},
			map[string]any{
				"id":    float64(2),
				"name":  "Item 2",
				"desc":  "Base item for filters",
				"price": 1.50,
				"tags":  []any{"four", "five", "six"},
			},
		},
	}

	return benchmarkCase{
		name:       "simple_lookup",
		input:      input,
		expected:   "Item 1",
		shorthand:  "items[1].name",
		jmespath:   "items[1].name",
		jq:         ".items[1].name",
		sourceNote: "Adapted from existing shorthand GetPath benchmarks.",
	}
}

func makeComplexProjectionCase() benchmarkCase {
	input := map[string]any{
		"items": []any{
			float64(0),
			map[string]any{
				"id":    float64(1),
				"name":  "Item 1",
				"desc":  "Base item for simple projections",
				"price": 4.99,
				"tags":  []any{"one", "two", "three"},
			},
			map[string]any{
				"id":    float64(2),
				"name":  "Item 2",
				"desc":  "Base item for filters",
				"price": 1.50,
				"tags":  []any{"four", "five", "six"},
			},
		},
	}

	return benchmarkCase{
		name:  "complex_filter_projection",
		input: input,
		expected: map[string]any{
			"name":         "Item 2",
			"price":        1.50,
			"filteredTags": []any{"four", "five"},
		},
		shorthand:  "items[-1].{name, price, filteredTags: tags[@ startsWith f]}",
		jmespath:   "items[-1].{name: name, price: price, filteredTags: tags[?starts_with(@, 'f')]}",
		jq:         `.items[-1] | {name, price, filteredTags: [.tags[] | select(startswith("f"))]}`,
		sourceNote: "Adapted from existing shorthand complex GetPath benchmarks.",
	}
}

func makeRestishCollectionCase() benchmarkCase {
	items := make([]any, 0, 96)
	expected := make([]any, 0, 32)

	for i := 0; i < 96; i++ {
		status := "pending"
		if i%3 == 0 {
			status = "ready"
		} else if i%3 == 1 {
			status = "retrying"
		}

		item := map[string]any{
			"id":     fmt.Sprintf("req-%03d", i),
			"status": status,
			"title":  fmt.Sprintf("Request %03d", i),
			"_links": map[string]any{
				"self": map[string]any{
					"href": fmt.Sprintf("https://api.example.test/requests/%03d", i),
				},
				"describedby": map[string]any{
					"href": fmt.Sprintf("https://docs.example.test/requests/%03d", i),
				},
			},
			"content": map[string]any{
				"author": map[string]any{
					"login": fmt.Sprintf("user-%02d", i%7),
				},
				"tags": []any{
					fmt.Sprintf("team-%d", i%5),
					fmt.Sprintf("tier-%d", (i%3)+1),
				},
				"stats": map[string]any{
					"response_ms": float64(90 + (i % 25)),
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

	input := map[string]any{
		"items": items,
		"_links": map[string]any{
			"self": map[string]any{
				"href": "https://api.example.test/requests",
			},
			"next": map[string]any{
				"href": "https://api.example.test/requests?page=2",
			},
		},
	}

	return benchmarkCase{
		name:       "restish_collection_projection",
		input:      input,
		expected:   expected,
		shorthand:  "items[status == ready].{id, title, self: _links.self.href, author: content.author.login, tags: content.tags, latency: content.stats.response_ms}",
		jmespath:   "items[?status=='ready'].{id: id, title: title, self: _links.self.href, author: content.author.login, tags: content.tags, latency: content.stats.response_ms}",
		jq:         `[.items[] | select(.status == "ready") | {id, title, self: ._links.self.href, author: .content.author.login, tags: .content.tags, latency: .content.stats.response_ms}]`,
		sourceNote: "Inspired by Restish/HAL-style collection responses with links, embedded metadata, and field projection.",
	}
}

func makeAWSVolumesCase() benchmarkCase {
	volumes := make([]any, 0, 120)
	expected := make([]any, 0, 40)

	for i := 0; i < 120; i++ {
		volumeType := "gp2"
		if i%3 == 0 {
			volumeType = "gp3"
		} else if i%3 == 1 {
			volumeType = "io2"
		}

		attachments := []any{}
		if i%4 != 0 {
			attachments = append(attachments, map[string]any{
				"InstanceId": fmt.Sprintf("i-%08d", 10000000+i),
				"State":      "attached",
			})
		}

		tags := []any{
			map[string]any{"Key": "Name", "Value": fmt.Sprintf("volume-%03d", i)},
			map[string]any{"Key": "Env", "Value": []string{"prod", "stage", "dev"}[i%3]},
		}

		volume := map[string]any{
			"VolumeId":         fmt.Sprintf("vol-%08d", 50000000+i),
			"VolumeType":       volumeType,
			"AvailabilityZone": fmt.Sprintf("us-west-2%c", 'a'+rune(i%3)),
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

	input := map[string]any{
		"Volumes": volumes,
	}

	return benchmarkCase{
		name:       "aws_describe_volumes_projection",
		input:      input,
		expected:   expected,
		shorthand:  "Volumes[VolumeType == gp3].{id: VolumeId, az: AvailabilityZone, attachedInstance: Attachments.InstanceId|[0], name: Tags[Key == Name].Value|[0]}",
		jmespath:   "Volumes[?VolumeType=='gp3'].{id: VolumeId, az: AvailabilityZone, attachedInstance: Attachments[].InstanceId | [0], name: Tags[?Key=='Name'].Value | [0]}",
		jq:         `[.Volumes[] | select(.VolumeType == "gp3") | {id: .VolumeId, az: .AvailabilityZone, attachedInstance: (.Attachments[0].InstanceId // null), name: ([.Tags[] | select(.Key == "Name") | .Value] | .[0])}]`,
		sourceNote: "Inspired by AWS CLI --query usage against describe-volumes style payloads.",
	}
}
