package shorthand

import (
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	modifierNone = iota
	modifierString
)

// DeepAssign recursively merges a source map into the target.
func DeepAssign(target, source map[string]interface{}) {
	for k, v := range source {
		if vm, ok := v.(map[string]interface{}); ok {
			if _, ok := target[k]; ok {
				if tkm, ok := target[k].(map[string]interface{}); ok {
					DeepAssign(tkm, vm)
				} else {
					target[k] = vm
				}
			} else {
				target[k] = vm
			}
		} else {
			target[k] = v
		}
	}
}

func ConvertMapString(value interface{}) interface{} {
	switch tmp := value.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{}, len(tmp))
		for k, v := range tmp {
			m[fmt.Sprintf("%v", k)] = ConvertMapString(v)
		}
		return m
	case map[string]interface{}:
		for k, v := range tmp {
			tmp[k] = ConvertMapString(v)
		}
	case []interface{}:
		for i, v := range tmp {
			tmp[i] = ConvertMapString(v)
		}
	}

	return value
}

// GetInput loads data from stdin (if present) and from the passed arguments,
// returning the final structure.
func GetInput(args []string) (interface{}, error) {
	stat, _ := os.Stdin.Stat()
	return getInput(stat.Mode(), os.Stdin, args, ParseOptions{
		EnableFileInput:       true,
		EnableObjectDetection: true,
	})
}

func GetInputWithOptions(args []string, options ParseOptions) (interface{}, error) {
	stat, _ := os.Stdin.Stat()
	return getInput(stat.Mode(), os.Stdin, args, options)
}

func getInput(mode fs.FileMode, stdinFile io.Reader, args []string, options ParseOptions) (interface{}, error) {
	var stdin interface{}

	if (mode & os.ModeCharDevice) == 0 {
		d, err := ioutil.ReadAll(stdinFile)
		if err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(d, &stdin); err != nil {
			if len(args) > 0 {
				return nil, err
			}
			return nil, err
		}
	}

	if len(args) == 0 {
		return stdin, nil
	}

	d := Document{
		options: options,
	}
	if err := d.Parse(strings.Join(args, " ")); err != nil {
		return nil, err
	}

	data, err := d.Apply(stdin)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func Parse(input string, options ParseOptions, existing interface{}) (interface{}, Error) {
	d := Document{options: options}
	if err := d.Parse(input); err != nil {
		return nil, err
	}
	return d.Apply(existing)
}

// Get the shorthand representation of an input map.
func Get(input map[string]interface{}) string {
	result := renderValue(true, input)
	return result[1 : len(result)-1]
}

func renderValue(start bool, value interface{}) string {
	// Go uses `<nil>` so here we hard-code `null` to match JSON/YAML.
	if value == nil {
		return ": null"
	}

	switch v := value.(type) {
	case map[string]interface{}:
		// Special case: foo.bar: 1
		if !start && len(v) == 1 {
			for k := range v {
				return "." + k + renderValue(false, v[k])
			}
		}

		// Normal case: foo{a: 1, b: 2}
		var keys []string

		for k := range v {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		var fields []string
		for _, k := range keys {
			fields = append(fields, k+renderValue(false, v[k]))
		}

		return "{" + strings.Join(fields, ", ") + "}"
	case []interface{}:
		var items []string

		// Special case: foo: [1, 2, 3]
		scalars := true
		for _, item := range v {
			switch item.(type) {
			case map[string]interface{}:
				scalars = false
			case []interface{}:
				scalars = false
			}
		}

		if scalars {
			for _, item := range v {
				items = append(items, fmt.Sprintf("%v", item))
			}

			return ": [" + strings.Join(items, ", ") + "]"
		}

		// Normal case: foo[]: 1, []{id: 1, count: 2}
		for _, item := range v {
			items = append(items, "[]"+renderValue(false, item))
		}

		return strings.Join(items, ", ")
	default:
		modifier := ""

		if s, ok := v.(string); ok {
			_, err := strconv.ParseFloat(s, 64)

			if err == nil || s == "null" || s == "true" || s == "false" {
				modifier = "~"
			}

			if len(s) > 50 || strings.Contains(s, "\n") {
				v = "@file"
			}
		}

		return fmt.Sprintf(":%s %v", modifier, v)
	}
}
