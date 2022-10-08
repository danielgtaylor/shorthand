package shorthand

import (
	"strconv"
	"strings"

	"github.com/danielgtaylor/mexpr"
)

type GetOptions struct {
	// DebugLogger sets a function to be used for printing out debug information.
	DebugLogger func(format string, a ...interface{})
}

func GetPath(path string, input any, options GetOptions) (any, bool, Error) {
	d := Document{
		expression: path,
		options: ParseOptions{
			DebugLogger: options.DebugLogger,
		},
	}
	result := input
	var ok bool
	var err Error
	for d.pos < uint(len(d.expression)) {
		result, ok, err = d.getPath(result)
		if err != nil {
			return result, ok, err
		}
		if d.peek() == '|' {
			d.next()
		}
	}
	return result, ok, nil
}

func (d *Document) parseUntil(open int, terminators ...rune) (quoted bool, canSlice bool, err Error) {
	canSlice = true
	d.buf.Reset()

outer:
	for {
		p := d.peek()
		if p == -1 {
			break outer
		}

		if p == '[' || p == '{' {
			open++
		} else if p == ']' || p == '}' {
			open--
			if open == 0 {
				break outer
			}
		}

		for _, t := range terminators {
			if p == t {
				break outer
			}
		}

		r := d.next()

		if r == '\\' {
			if d.parseEscape(false, false) {
				canSlice = false
				continue
			}
		}

		if r == '"' {
			if err = d.parseQuoted(false); err != nil {
				return
			}
			quoted = true
			continue
		}

		d.buf.WriteRune(r)
	}

	return
}

func (d *Document) parsePathIndex() (bool, int, int, string, Error) {
	d.skipWhitespace()
	start := d.pos
	_, canSlice, err := d.parseUntil(1, '|')
	if err != nil {
		return false, 0, 0, "", err
	}

	var value string
	if canSlice {
		value = d.expression[start:d.pos]
	} else {
		value = d.buf.String()
	}

	if !d.expect(']') {
		return false, 0, 0, "", d.error(d.pos-start, "expected ']' after index or filter")
	}

	if len(value) > 0 {
		indexes := strings.Split(value, ":")
		if len(indexes) == 1 {
			if index, err := strconv.Atoi(value); err == nil {
				return false, index, index, "", nil
			}
		} else {
			if startIndex, err := strconv.Atoi(indexes[0]); err == nil {
				if stopIndex, err := strconv.Atoi(indexes[1]); err == nil {
					return true, startIndex, stopIndex, "", nil
				}
			}
		}
	}

	return false, 0, 0, value, nil
}

func (d *Document) getFiltered(expr string, input any) (any, Error) {
	ast, err := mexpr.Parse(expr, nil)
	if err != nil {
		return nil, NewError(&d.expression, d.pos+uint(err.Offset()), uint(err.Length()), err.Error())
	}
	interpreter := mexpr.NewInterpreter(ast, mexpr.UnquotedStrings)
	savedPos := d.pos

	if s, ok := input.([]any); ok {
		results := []any{}
		for _, item := range s {
			result, err := interpreter.Run(item)
			if err != nil {
				continue
			}
			if b, ok := result.(bool); ok && b {
				out := item

				var err Error
				d.pos = savedPos
				out, _, err = d.getPath(item)
				if err != nil {
					return nil, err
				}
				results = append(results, out)
			}
		}
		return results, nil
	}
	return nil, nil
}

func (d *Document) getIndex2(input any) (any, Error) {
	isSlice, startIndex, stopIndex, expr, err := d.parsePathIndex()
	if err != nil {
		return nil, err
	}

	if expr != "" {
		return d.getFiltered(expr, input)
	}

	if s, ok := input.([]any); ok {
		if startIndex > len(s)-1 || stopIndex > len(s)-1 {
			return nil, nil
		}
		for startIndex < 0 {
			startIndex += len(s)
		}
		for stopIndex < 0 {
			stopIndex += len(s)
		}

		if !isSlice {
			return s[startIndex], nil
		}

		return s[startIndex : stopIndex+1], nil
	}

	return nil, nil
}

func (d *Document) parseProp2() (any, Error) {
	d.skipWhitespace()
	start := d.pos
	quoted, canSlice, err := d.parseUntil(0, '.', '[', '|', ',', '}')
	if err != nil {
		return nil, err
	}

	var key string
	if canSlice {
		key = d.expression[start:d.pos]
	} else {
		key = d.buf.String()
	}

	if !d.options.ForceStringKeys && !quoted {
		if v, ok := coerceValue(key); ok {
			return v, nil
		}
	}

	return key, nil
}

func (d *Document) getProp(input any) (any, bool, Error) {
	if s, ok := input.([]any); ok {
		var err Error
		savedPos := d.pos
		out := make([]any, len(s))

		for i := range s {
			d.pos = savedPos
			out[i], _, err = d.getPath(s[i])
			if err != nil {
				return nil, false, err
			}
		}

		return out, true, nil
	}

	key, err := d.parseProp2()
	if err != nil {
		return nil, false, err
	}

	if d.options.DebugLogger != nil {
		d.options.DebugLogger("Getting key '%v'", key)
	}

	if m, ok := input.(map[string]any); ok {
		if s, ok := key.(string); ok {
			v, ok := m[s]
			return v, ok, nil
		}
	} else if m, ok := input.(map[any]any); ok {
		v, ok := m[key]
		return v, ok, nil
	}

	return nil, false, nil
}

func (d *Document) flatten(input any) (any, Error) {
	if s, ok := input.([]any); ok {
		out := make([]any, 0, len(s))

		for _, item := range s {
			if !isArray(item) {
				out = append(out, item)
				continue
			}

			out = append(out, item.([]any)...)
		}
		return out, nil
	}
	return nil, nil
}

func (d *Document) getPath(input any) (any, bool, Error) {
	var err Error
	found := false

outer:
	for {
		switch d.peek() {
		case -1, '|':
			break outer
		case '[':
			d.next()
			if d.peek() == ']' {
				// Special case: flatten one level
				// [[1, 2], 3, [[4]]] => [1, 2, 3, [4]]
				d.next()
				input, err = d.flatten(input)
				if err != nil {
					return nil, false, err
				}
				found = true
				continue
			}
			input, err = d.getIndex2(input)
			if err != nil {
				return nil, false, err
			}
			found = true
		case '.':
			d.next()
			continue
		case '{':
			d.next()
			input, err = d.getFields(input)
			if err != nil {
				return nil, false, err
			}
			found = true
			continue
		default:
			input, found, err = d.getProp(input)
			if err != nil {
				return nil, false, err
			}
		}
	}

	return input, found, nil
}

func (d *Document) getFields(input any) (any, Error) {
	d.buf.Reset()
	if !isMap(input) {
		return nil, d.error(1, "field selection requires a map, but found %v", input)
	}
	result := map[string]any{}
	key := ""
	open := 1
	var r rune
	d.skipWhitespace()
	for {
		r = d.next()
		if r == '"' {
			d.buf.WriteRune('"')
			if err := d.parseQuoted(true); err != nil {
				return nil, err
			}
			d.buf.WriteRune('"')
			continue
		}
		if r == '\\' {
			if d.parseEscape(false, true) {
				continue
			}
		}
		if r == -1 {
			break
		}
		if r == ':' {
			key = d.buf.String()
			d.buf.Reset()
			d.skipWhitespace()
			continue
		}
		if r == '{' {
			open++
		}
		if r == '}' {
			open--
		}
		if r == ',' || open == 0 {
			path := d.buf.String()
			if m, ok := input.(map[string]any); ok {
				if key == "" {
					result[path] = m[path]
				} else {
					expr, pos := d.expression, d.pos
					tmp, _, err := GetPath(path, input, GetOptions{
						DebugLogger: d.options.DebugLogger,
					})
					d.expression, d.pos = expr, pos
					if err != nil {
						return nil, err
					}
					result[key] = tmp
				}
			}
			if r == '}' {
				break
			}
			key = ""
			d.buf.Reset()
			d.skipWhitespace()
			continue
		}
		d.buf.WriteRune(r)
	}
	return result, nil
}
