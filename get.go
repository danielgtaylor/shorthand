package shorthand

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/danielgtaylor/mexpr"
)

type fieldSpec struct {
	key  string
	path string
}

// compiledPathCache memoizes parsed query plans keyed by the original query
// string. Compiled queries are immutable after creation and safe to share
// across goroutines.
var compiledPathCache sync.Map

// mexprCache stores parsed filter ASTs keyed by expression string. The
// resulting tree is read-only during execution so it is safe to share.
var mexprCache sync.Map

// compiledQuery is the internal prepared-query representation used by GetPath.
// A query may contain multiple pipe-separated segments, each executed in order.
type compiledQuery struct {
	expression string
	segments   []compiledSegment
}

type compiledSegment struct {
	ops []compiledOp
}

type compiledOp interface{}

type compiledDotOp struct{}

type compiledFlattenOp struct{}

type compiledPropOp struct {
	key any
}

type compiledRecursivePropOp struct {
	key any
}

type compiledIndexOp struct {
	isSlice    bool
	startIndex int
	stopIndex  int
}

type compiledFilterOp struct {
	ast *mexpr.Node
}

type compiledField struct {
	key   string
	query *compiledQuery
}

type compiledFieldsOp struct {
	offset   uint
	fields   []compiledField
	parseErr Error
}

type compiledExecResult struct {
	value    any
	found    bool
	consumed bool
}

type GetOptions struct {
	// DebugLogger sets a function to be used for printing out debug information.
	DebugLogger func(format string, a ...any)
}

var propPathUnescaper = strings.NewReplacer(`\.`, ".", `\{`, "{", `\[`, "[", `\]`, "]", `\:`, ":", `\^`, "^")

// unescapePropPath removes prop-escaping backslashes added by parseQuoted(escapeProp=true).
func unescapePropPath(s string) string {
	return propPathUnescaper.Replace(s)
}

// mapKeys returns the keys of the map m.
// The keys will be in an indeterminate order.
func mapKeys[M ~map[K]V, K comparable, V any](m M) []K {
	r := make([]K, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	return r
}

func GetPath(path string, input any, options GetOptions) (any, bool, Error) {
	query, err := getCompiledPath(path)
	if err != nil {
		return nil, false, err
	}
	return query.Exec(input, options)
}

// getCompiledPath returns a cached compiled query plan or builds one on first
// use. Callers should treat the returned plan as read-only.
func getCompiledPath(path string) (*compiledQuery, Error) {
	if cached, ok := compiledPathCache.Load(path); ok {
		return cached.(*compiledQuery), nil
	}

	query, err := compilePath(path)
	if err != nil {
		return nil, err
	}

	actual, _ := compiledPathCache.LoadOrStore(path, query)
	return actual.(*compiledQuery), nil
}

// compilePath parses a query string into executable segments. Parsing reuses
// the Document helpers below so the compiled path stays aligned with the query
// grammar and existing error reporting.
func compilePath(path string) (*compiledQuery, Error) {
	d := Document{expression: path}
	query := &compiledQuery{expression: path}

	for d.pos < uint(len(d.expression)) {
		segment, err := d.compileSegment()
		if err != nil {
			return nil, err
		}
		query.segments = append(query.segments, segment)
		if d.peek() == '|' {
			d.next()
		}
	}

	return query, nil
}

// compileSegment compiles one pipe-delimited segment of a query into a flat
// sequence of executable ops. Filters and field selections recursively compile
// nested query fragments.
func (d *Document) compileSegment() (compiledSegment, Error) {
	ops := make([]compiledOp, 0, 8)

outer:
	for {
		switch d.peek() {
		case -1, '|':
			break outer
		case '[':
			d.next()
			if d.peek() == ']' {
				d.next()
				ops = append(ops, compiledFlattenOp{})
				continue
			}

			isSlice, startIndex, stopIndex, expr, err := d.parsePathIndex()
			if err != nil {
				return compiledSegment{}, err
			}

			if expr != "" {
				ast, err := compileMexpr(d, expr)
				if err != nil {
					return compiledSegment{}, err
				}
				ops = append(ops, compiledFilterOp{ast: ast})
				continue
			}

			ops = append(ops, compiledIndexOp{
				isSlice:    isSlice,
				startIndex: startIndex,
				stopIndex:  stopIndex,
			})
		case '.':
			d.next()
			if d.peek() == '.' {
				d.next()
				key, err := d.parseGetProp()
				if err != nil {
					return compiledSegment{}, err
				}
				ops = append(ops, compiledRecursivePropOp{key: key})
				continue
			}
			ops = append(ops, compiledDotOp{})
		case '{':
			start := d.pos
			d.next()
			fields, _, err := d.parseFieldSpecs()
			if err != nil {
				ops = append(ops, compiledFieldsOp{
					offset:   start - 1,
					parseErr: err,
				})
				break outer
			}

			compiledFields := make([]compiledField, len(fields))
			for i, field := range fields {
				query, err := getCompiledPath(field.path)
				if err != nil {
					return compiledSegment{}, err
				}
				compiledFields[i] = compiledField{
					key:   field.key,
					query: query,
				}
			}

			ops = append(ops, compiledFieldsOp{
				offset: start - 1,
				fields: compiledFields,
			})
		case ',', ']', '}':
			d.next()
		default:
			key, err := d.parseGetProp()
			if err != nil {
				return compiledSegment{}, err
			}
			ops = append(ops, compiledPropOp{key: key})
		}
	}

	return compiledSegment{ops: ops}, nil
}

// compileMexpr compiles and caches filter expressions used inside `[...]`.
func compileMexpr(d *Document, expr string) (*mexpr.Node, Error) {
	if cached, ok := mexprCache.Load(expr); ok {
		return cached.(*mexpr.Node), nil
	}

	ast, err := mexpr.Parse(expr, nil)
	if err != nil {
		return nil, NewError(&d.expression, d.pos-uint(len(expr)+1)+uint(err.Offset()), uint(err.Length()), err.Error())
	}

	actual, _ := mexprCache.LoadOrStore(expr, ast)
	return actual.(*mexpr.Node), nil
}

// Exec evaluates a compiled query against an input value while preserving the
// same `(value, found, err)` contract as GetPath.
func (q *compiledQuery) Exec(input any, options GetOptions) (any, bool, Error) {
	result := input
	found := false

	for i := range q.segments {
		execResult, err := q.execSegment(&q.segments[i], result, 0, options)
		if err != nil {
			return execResult.value, execResult.found, err
		}
		result = execResult.value
		found = execResult.found
	}

	return result, found, nil
}

// execSegment executes a compiled segment, optionally starting in the middle of
// the op list. That ability lets filter and dot-fanout ops apply the remaining
// tail of the segment to each matching array item without reparsing the query.
func (q *compiledQuery) execSegment(segment *compiledSegment, input any, start int, options GetOptions) (compiledExecResult, Error) {
	result := input
	found := false
	consumed := false

	for i := start; i < len(segment.ops); i++ {
		switch op := segment.ops[i].(type) {
		case compiledDotOp:
			consumed = true

			items, ok := result.([]any)
			if !ok {
				continue
			}

			out := make([]any, 0, len(items))
			for _, item := range items {
				child, err := q.execSegment(segment, item, i+1, options)
				if err != nil {
					return compiledExecResult{}, err
				}
				if child.consumed {
					if child.found {
						out = append(out, child.value)
					}
				} else if child.value != nil {
					out = append(out, child.value)
				}
			}

			return compiledExecResult{value: out, found: true, consumed: true}, nil
		case compiledFlattenOp:
			if options.DebugLogger != nil {
				options.DebugLogger("Flattening %v", result)
			}
			if items, ok := result.([]any); ok {
				out := make([]any, 0, len(items))
				for _, item := range items {
					if !isArray(item) {
						out = append(out, item)
						continue
					}
					out = append(out, item.([]any)...)
				}
				result = out
			} else {
				result = nil
			}
			found = true
			consumed = true
		case compiledPropOp:
			var ok bool
			result, ok = execCompiledProp(op.key, result, options)
			found = ok
			consumed = true
		case compiledRecursivePropOp:
			var err Error
			result, err = execCompiledRecursiveProp(op.key, result, options)
			if err != nil {
				return compiledExecResult{}, err
			}
			found = true
			consumed = true
		case compiledIndexOp:
			result = execCompiledIndex(op, result, options)
			found = true
			consumed = true
		case compiledFilterOp:
			consumed = true
			items, ok := result.([]any)
			if !ok {
				result = nil
				found = true
				continue
			}

			interpreter := mexpr.NewInterpreter(op.ast, mexpr.UnquotedStrings)
			out := make([]any, 0, len(items))
			for _, item := range items {
				filterResult, err := interpreter.Run(item)
				if err != nil {
					continue
				}
				if matched, ok := filterResult.(bool); ok && matched {
					child, err := q.execSegment(segment, item, i+1, options)
					if err != nil {
						return compiledExecResult{}, err
					}
					out = append(out, child.value)
				}
			}

			return compiledExecResult{value: out, found: true, consumed: true}, nil
		case compiledFieldsOp:
			if !isMap(result) {
				return compiledExecResult{}, NewError(&q.expression, op.offset, 1, "field selection requires a map, but found %v", result)
			}
			if op.parseErr != nil {
				return compiledExecResult{}, op.parseErr
			}

			out := make(map[string]any, len(op.fields))
			for _, field := range op.fields {
				value, _, err := field.query.Exec(result, options)
				if err != nil {
					return compiledExecResult{}, err
				}
				out[field.key] = value
			}
			result = out
			found = true
			consumed = true
		}
	}

	return compiledExecResult{value: result, found: found, consumed: consumed}, nil
}

// execCompiledProp resolves a single property access or wildcard against a map.
func execCompiledProp(key any, input any, options GetOptions) (any, bool) {
	if options.DebugLogger != nil {
		options.DebugLogger("Getting key '%v'", key)
	}

	if m, ok := input.(map[string]any); ok {
		if s, ok := key.(string); ok {
			if s == "*" {
				keys := mapKeys(m)
				sort.Strings(keys)
				values := make([]any, len(m))
				for i, k := range keys {
					values[i] = m[k]
				}
				return values, true
			}

			v, ok := m[s]
			return v, ok
		}
	} else if m, ok := input.(map[any]any); ok {
		if s, ok := key.(string); ok && s == "*" {
			keys := make([]any, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(i, j int) bool {
				return fmt.Sprintf("%v", keys[i]) < fmt.Sprintf("%v", keys[j])
			})
			values := make([]any, len(m))
			for i, k := range keys {
				values[i] = m[k]
			}
			return values, true
		}

		v, ok := m[key]
		return v, ok
	}

	if options.DebugLogger != nil {
		options.DebugLogger("Cannot get key %v from input %v", key, input)
	}

	return nil, false
}

// execCompiledRecursiveProp performs recursive descent (`..field`) against the
// input tree while preserving the stable ordering expected by existing tests.
func execCompiledRecursiveProp(key, input any, options GetOptions) ([]any, Error) {
	if options.DebugLogger != nil {
		options.DebugLogger("Recursive getting key '%v'", key)
	}
	return execCompiledFindPropRecursive(key, input)
}

func execCompiledFindPropRecursive(key, input any) ([]any, Error) {
	var results []any

	if m, ok := input.(map[string]any); ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := m[k]
			if s, ok := key.(string); ok && k == s {
				results = append(results, v)
			}
			if nested, err := execCompiledFindPropRecursive(key, v); err == nil {
				results = append(results, nested...)
			}
		}
	}

	if m, ok := input.(map[any]any); ok {
		for k, v := range m {
			if k == key {
				results = append(results, v)
			}
			if nested, err := execCompiledFindPropRecursive(key, v); err == nil {
				results = append(results, nested...)
			}
		}
	}

	if items, ok := input.([]any); ok {
		for _, item := range items {
			if nested, err := execCompiledFindPropRecursive(key, item); err == nil {
				results = append(results, nested...)
			}
		}
	}

	return results, nil
}

// execCompiledIndex applies array/string/byte indexing and slicing semantics
// for a pre-parsed index operation.
func execCompiledIndex(op compiledIndexOp, input any, options GetOptions) any {
	if options.DebugLogger != nil {
		options.DebugLogger("Getting index %v:%v ", op.startIndex, op.stopIndex)
	}

	length := 0
	switch value := input.(type) {
	case string:
		length = utf8.RuneCountInString(value)
	case []byte:
		length = len(value)
	case []any:
		length = len(value)
	}

	startIndex := op.startIndex
	stopIndex := op.stopIndex
	if startIndex < 0 {
		startIndex += length
	}
	if stopIndex < 0 {
		stopIndex += length
	}
	if stopIndex > length-1 {
		stopIndex = length - 1
	}
	if startIndex < 0 || startIndex > length-1 || stopIndex < 0 || startIndex > stopIndex {
		return nil
	}

	switch value := input.(type) {
	case string:
		return stringRuneSlice(value, startIndex, stopIndex)
	case []byte:
		if !op.isSlice {
			return value[startIndex]
		}
		return value[startIndex : stopIndex+1]
	case []any:
		if !op.isSlice {
			return value[startIndex]
		}
		return value[startIndex : stopIndex+1]
	default:
		return nil
	}
}

func (d *Document) parseUntil(open int, terminators ...rune) (quoted bool, canSlice bool, err Error) {
	d.buf.Reset()
	return d.parseUntilNoReset(open, terminators...)
}

func (d *Document) parseUntilNoReset(open int, terminators ...rune) (quoted bool, canSlice bool, err Error) {
	canSlice = true
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

// parsePathIndex parses the contents of `[...]` after the opening `[` has
// already been consumed. It returns either an index/slice description or a
// filter expression string for the compiled query engine to handle.
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
			if indexes[0] == "" {
				indexes[0] = "0"
			}
			if startIndex, err := strconv.Atoi(indexes[0]); err == nil {
				if indexes[1] == "" {
					indexes[1] = "-1"
				}
				if stopIndex, err := strconv.Atoi(indexes[1]); err == nil {
					return true, startIndex, stopIndex, "", nil
				}
			}
		}

		if value[0] == '?' {
			value = value[1:]
		}
	}

	return false, 0, 0, value, nil
}

func stringRuneSlice(s string, startIndex int, stopIndex int) string {
	byteStart := 0
	byteEnd := len(s)
	runeIndex := 0

	for i := range s {
		if runeIndex == startIndex {
			byteStart = i
		}
		if runeIndex == stopIndex+1 {
			byteEnd = i
			break
		}
		runeIndex++
	}

	return s[byteStart:byteEnd]
}

func (d *Document) parseGetProp() (any, Error) {
	d.skipWhitespace()
	start := d.pos
	quoted, canSlice, err := d.parseUntil(0, '.', '[', '|', ',', '}', ']')
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
		if v, ok := coerceValue(key, false); ok {
			return v, nil
		}
	}

	return key, nil
}

// parseFieldSpecs parses the contents of `{...}` after the opening `{` has
// already been consumed. It returns field aliases and their child path strings
// so the compiled query engine can recursively compile them.
func (d *Document) parseFieldSpecs() ([]fieldSpec, uint, Error) {
	d.buf.Reset()
	start := d.pos - 1
	key := ""
	open := 1
	var r rune
	d.skipWhitespace()
	fields := make([]fieldSpec, 0, 4)
	for {
		r = d.next()
		if r == '"' {
			if err := d.parseQuoted(true); err != nil {
				return nil, 0, err
			}
			continue
		}
		if r == '\\' {
			if d.parseEscape(false, true) {
				continue
			}
		}
		if r == -1 {
			return nil, 0, d.error(1, "expected '}' to close field selection")
		}
		if r == '[' {
			d.buf.WriteRune(r)
			if _, _, err := d.parseUntilNoReset(1, '|'); err != nil {
				return nil, 0, err
			}
			continue
		}
		if r == ':' && open <= 1 {
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
		if open == 0 || (open == 1 && r == ',') {
			path := d.buf.String()
			if key == "" {
				// Use the unescaped path as the output key so that quoted names
				// like `{"foo.bar"}` produce key "foo.bar", not "foo\.bar".
				key = unescapePropPath(path)
			}
			fields = append(fields, fieldSpec{key: key, path: path})
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
	return fields, d.pos - start, nil
}
