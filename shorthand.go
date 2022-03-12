package shorthand

import (
	"encoding/base64"
	"encoding/json"
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

//go:generate pigeon -o generated.go shorthand.peg

const (
	modifierNone = iota
	modifierString
)

func toIfaceSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	return v.([]interface{})
}

func repeatedWithIndex(v interface{}, index int, cb func(v interface{})) {
	for _, i := range v.([]interface{}) {
		cb(i.([]interface{})[index])
	}
}

// AST contains all of the key-value pairs in the document.
type AST []*KeyValue

// KeyValue groups a Key with the key's associated value.
type KeyValue struct {
	PostProcess bool
	Key         *Key
	Value       interface{}
}

// Key contains parts and key-specific configuration.
type Key struct {
	ResetContext bool
	Parts        []*KeyPart
}

// KeyPart has a name and optional indices.
type KeyPart struct {
	Key   string
	Index []int
}

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

// GetInput loads data from stdin (if present) and from the passed arguments,
// returning the final structure.
func GetInput(args []string) (map[string]interface{}, error) {
	stat, _ := os.Stdin.Stat()
	return getInput(stat.Mode(), os.Stdin, args)
}

func getInput(mode fs.FileMode, stdinFile io.Reader, args []string) (map[string]interface{}, error) {
	var stdin map[string]interface{}

	if (mode & os.ModeCharDevice) == 0 {
		d, err := ioutil.ReadAll(stdinFile)
		if err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(d, &stdin); err != nil {
			return nil, err
		}
	}

	if len(args) == 0 {
		return stdin, nil
	}

	parsed, err := ParseAndBuild("args", strings.Join(args, " "), stdin)
	if err != nil {
		return nil, err
	}

	return parsed, nil
}

// sliceRef represents a reference to a slice on a parent object (map or slice)
// that can be modified and have values set on it.
type sliceRef struct {
	Base  interface{}
	Index int
	Key   string
}

func (c *sliceRef) GetList() []interface{} {
	switch b := c.Base.(type) {
	case map[string]interface{}:
		if l, ok := b[c.Key].([]interface{}); ok {
			return l
		}
	case []interface{}:
		if l, ok := b[c.Index].([]interface{}); ok {
			return l
		}
	}
	return nil
}

func (c *sliceRef) Length() int {
	return len(c.GetList())
}

func (c *sliceRef) Grow(length int) {
	l := c.GetList()
	if l == nil {
		// This was not a list... might have been another type which we are
		// now going to overwrite.
		l = []interface{}{}
	}

	for len(l) < length+1 {
		l = append(l, nil)
	}

	switch b := c.Base.(type) {
	case map[string]interface{}:
		b[c.Key] = l
	case []interface{}:
		b[c.Index] = l
	}
}

func (c *sliceRef) GetValue(index int) interface{} {
	return c.GetList()[index]
}

func (c *sliceRef) SetValue(index int, value interface{}) {
	c.GetList()[index] = value
}

// ParseAndBuild takes a string and returns the structured data it represents.
func ParseAndBuild(filename, input string, existing ...map[string]interface{}) (map[string]interface{}, error) {
	parsed, err := Parse(filename, []byte(input))
	if err != nil {
		return nil, err
	}

	return Build(parsed.(AST), existing...)
}

// Build an AST of key-value pairs into structured data.
func Build(ast AST, existing ...map[string]interface{}) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	for _, e := range existing {
		DeepAssign(result, e)
	}
	ctx := result
	var ctxSlice sliceRef

	for _, kv := range ast {
		k := kv.Key
		v := kv.Value

		if subAST, ok := v.(AST); ok {
			// If the value is itself an AST, then recursively process it!
			parsed, err := Build(subAST)
			if err != nil {
				return result, err
			}
			v = parsed
		} else if vStr, ok := v.(string); ok && kv.PostProcess {
			// If the value is a string, then handle special cases here.
			if len(vStr) > 1 && strings.HasPrefix(vStr, "@") {
				filename := vStr[1:]

				forceString := false
				useBase64 := false
				if filename[0] == '~' {
					forceString = true
					filename = filename[1:]
				} else if filename[0] == '%' {
					forceString = true
					filename = filename[1:]
					useBase64 = true
				}
				data, err := ioutil.ReadFile(filename)
				if err != nil {
					return result, err
				}

				if !forceString && strings.HasSuffix(vStr, ".json") {
					// Try to load data from JSON file.
					var unmarshalled interface{}

					if err := json.Unmarshal(data, &unmarshalled); err != nil {
						return result, err
					}

					v = unmarshalled
				} else {
					if useBase64 {
						v = base64.StdEncoding.EncodeToString(data)
					} else {
						v = string(data)
					}
				}
			}
		}

		// Reset context to the root or keep going from where we left off.
		if k.ResetContext {
			ctx = result
		}

		for ki, kp := range k.Parts {
			// If there is a key, and the key is not in the current context, then it
			// must be created as either a list or map depending on whether there
			// are index items for one or more lists.
			if kp.Key != "" && (ki < len(k.Parts)-1 || len(kp.Index) > 0) {
				if len(kp.Index) > 0 {
					if ctx[kp.Key] == nil {
						ctx[kp.Key] = []interface{}{}
					}
					ctxSlice.Base = ctx
					ctxSlice.Key = kp.Key
				} else {
					if ctx[kp.Key] == nil {
						ctx[kp.Key] = make(map[string]interface{})
					}
					if _, ok := ctx[kp.Key].(map[string]interface{}); !ok {
						ctx[kp.Key] = make(map[string]interface{})
					}
					ctx = ctx[kp.Key].(map[string]interface{})
				}
			}

			// For each index item, create the associated list item and update the
			// context.
			for i, index := range kp.Index {
				if index == -1 {
					if ctxSlice.Base != nil {
						index = ctxSlice.Length()
					} else {
						index = 0
					}
				}

				ctxSlice.Grow(index)

				if i < len(kp.Index)-1 {
					newBase := ctxSlice.GetList()
					if len(newBase) < index+1 || newBase[index] == nil {
						newList := []interface{}{}
						ctxSlice.SetValue(index, newList)
					}
					ctxSlice.Index = index
					ctxSlice.Base = newBase
				} else {
					// This is the last index item. If it is also the last key part, then
					// set the value. Otherwise, create a map for the next key part to
					// use and update the context.
					if ki < len(k.Parts)-1 {
						if ctxSlice.GetValue(index) == nil {
							ctxSlice.SetValue(index, map[string]interface{}{})
						}
						ctx = ctxSlice.GetValue(index).(map[string]interface{})
					} else {
						ctxSlice.SetValue(index, v)
					}
				}
			}

			// If this is the last key part and has no list indexes, then just set
			// the value on the current context.
			if ki == len(k.Parts)-1 && len(kp.Index) == 0 {
				ctx[kp.Key] = v
				if _, ok := v.([]interface{}); ok {
					ctxSlice.Base = v
					ctxSlice.Index = ki
				}
			}
		}
	}

	return result, nil
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

		// Special case: foo: 1, 2, 3
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

			return ": " + strings.Join(items, ", ")
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
