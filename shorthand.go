package shorthand

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

var ErrInvalidFile = errors.New("file cannot be parsed as structured data as it contains invalid UTF-8 characters")

func ConvertMapString(value any) any {
	switch tmp := value.(type) {
	case map[any]any:
		m := make(map[string]any, len(tmp))
		for k, v := range tmp {
			m[fmt.Sprintf("%v", k)] = ConvertMapString(v)
		}
		return m
	case map[string]any:
		for k, v := range tmp {
			tmp[k] = ConvertMapString(v)
		}
	case []any:
		for i, v := range tmp {
			tmp[i] = ConvertMapString(v)
		}
	}

	return value
}

// GetInput loads data from stdin (if present) and from the passed arguments,
// returning the final structure. Returns the result, whether the result is
// structured data (or raw file []byte), and if any errors occurred.
func GetInput(args []string, options ParseOptions) (any, bool, error) {
	stat, _ := os.Stdin.Stat()
	return getInput(stat.Mode(), os.Stdin, args, options)
}

func getInput(mode fs.FileMode, stdinFile io.Reader, args []string, options ParseOptions) (any, bool, error) {
	var stdin any

	if (mode & os.ModeCharDevice) == 0 {
		d, err := io.ReadAll(stdinFile)
		if err != nil {
			return nil, false, err
		}

		if len(args) == 0 {
			// No modification requested, just pass the raw file through.
			return d, false, nil
		}

		if !utf8.Valid(d) {
			return nil, false, ErrInvalidFile
		}

		result, err := Unmarshal(string(d), options, nil)
		if err != nil {
			return nil, false, err
		}
		stdin = result
	}

	if len(args) == 0 {
		return stdin, true, nil
	}

	result, err := Unmarshal(strings.Join(args, " "), options, stdin)
	return result, true, err
}

func Unmarshal(input string, options ParseOptions, existing any) (any, Error) {
	d := Document{options: options}
	return d.Unmarshal(input, existing)
}

type MarshalOptions struct {
	Indent  string
	Spacer  string
	UseFile bool
}

func (o MarshalOptions) GetIndent(level int) string {
	if o.Indent == "" {
		return ""
	}
	result := "\n"
	for i := 0; i < level; i++ {
		result += o.Indent
	}
	return result
}

func (o MarshalOptions) GetSeparator(level int) string {
	if o.Indent != "" {
		return o.GetIndent(level)
	}

	return "," + o.Spacer
}

var marshalString = json.Marshal

func quoteString(s string) string {
	b, err := marshalString(s)
	if err == nil {
		return string(b)
	}
	return strconv.Quote(strings.ToValidUTF8(s, "\ufffd"))
}

func containsAnyRune(s string, chars string) bool {
	for _, r := range s {
		if strings.ContainsRune(chars, r) {
			return true
		}
	}
	return false
}

func shouldQuoteKey(s string) bool {
	return s == "" ||
		canCoerce(s) ||
		strings.TrimSpace(s) != s ||
		strings.Contains(s, "//") ||
		containsAnyRune(s, "\".[]{}:^,\\")
}

func shouldQuoteStringValue(s string) bool {
	return s == "" ||
		s == "undefined" ||
		canCoerce(s) ||
		strings.TrimSpace(s) != s ||
		strings.HasPrefix(s, "@") ||
		strings.HasPrefix(s, "%") ||
		strings.Contains(s, "//") ||
		containsAnyRune(s, "\"[],{}\n\r\t\\")
}

func renderStringKey(s string) string {
	if shouldQuoteKey(s) {
		return quoteString(s)
	}
	return s
}

func renderMapKey(k any) string {
	if s, ok := k.(string); ok {
		return renderStringKey(s)
	}
	return fmt.Sprintf("%v", k)
}

func Marshal(input any, options ...MarshalOptions) string {
	if len(options) == 0 {
		options = []MarshalOptions{{}}
	}
	return renderValue(options[0], 0, false, input)
}

func MarshalCLI(input any) string {
	result := Marshal(input, MarshalOptions{Spacer: " ", UseFile: true})
	if strings.HasPrefix(result, "{") && result != "{}" {
		result = result[1 : len(result)-1]
	}
	return result
}

func MarshalPretty(input any) string {
	return Marshal(input, MarshalOptions{Spacer: " ", Indent: "  "})
}

func renderValue(options MarshalOptions, level int, fromKey bool, value any) string {
	prefix := ""
	if fromKey {
		prefix = ":" + options.Spacer
	}

	// Go uses `nil` so here we hard-code `null` to match JSON/YAML.
	if value == nil {
		return prefix + "null"
	}

	switch v := value.(type) {
	case map[any]any:
		// Special case: foo.bar: 1
		if len(v) == 1 {
			dot := ""
			if fromKey {
				dot = "."
			}
			for k := range v {
				return dot + renderMapKey(k) + renderValue(options, level, true, v[k])
			}
		}

		// Normal case: foo{a: 1, b: 2}
		var keys []any

		for k := range v {
			keys = append(keys, k)
		}

		sort.Slice(keys, func(i, j int) bool {
			return fmt.Sprintf("%v", keys[i]) < fmt.Sprintf("%v", keys[j])
		})

		var fields []string
		for _, k := range keys {
			fields = append(fields, renderMapKey(k)+renderValue(options, level+1, true, v[k]))
		}

		return "{" + options.GetIndent(level+1) + strings.Join(fields, options.GetSeparator(level+1)) + options.GetIndent(level) + "}"
	case map[string]any:
		// Special case: foo.bar: 1
		if len(v) == 1 {
			dot := ""
			if fromKey {
				dot = "."
			}
			for k := range v {
				return dot + renderStringKey(k) + renderValue(options, level, true, v[k])
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
			fields = append(fields, renderStringKey(k)+renderValue(options, level+1, true, v[k]))
		}

		return "{" + options.GetIndent(level+1) + strings.Join(fields, options.GetSeparator(level+1)) + options.GetIndent(level) + "}"
	case []any:
		var items []string

		// Normal case: foo: [1, true, {id: 1, count: 2}]
		for _, item := range v {
			items = append(items, renderValue(options, level+1, false, item))
		}

		return prefix + "[" + options.GetIndent(level+1) + strings.Join(items, options.GetSeparator(level+1)) + options.GetIndent(level) + "]"
	default:
		if s, ok := v.(string); ok {
			if options.UseFile && (len(s) > 50 || strings.Contains(s, "\n")) {
				// Long strings are represented as being loaded from files.
				v = "@file"
			} else if shouldQuoteStringValue(s) {
				v = quoteString(s)
			}
		}

		return fmt.Sprintf("%s%v", prefix, v)
	}
}
