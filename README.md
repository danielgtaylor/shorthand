# Structured Data Shorthand Syntax

[![Docs](https://godoc.org/github.com/danielgtaylor/shorthand?status.svg)](https://pkg.go.dev/github.com/danielgtaylor/shorthand?tab=doc) [![Go Report Card](https://goreportcard.com/badge/github.com/danielgtaylor/shorthand)](https://goreportcard.com/report/github.com/danielgtaylor/shorthand)

Shorthand is a superset and friendlier variant of JSON designed with several use-cases in mind:

| Use Case             | Example                                          |
| -------------------- | ------------------------------------------------ |
| CLI arguments/input  | `my-cli post 'foo.bar[0]{baz: 1, hello: world}'` |
| Patch operations     | `name: undefined, item.tags[]: appended`         |
| Query language       | `items[created before "2022-01-01"].{id, tags}`  |
| Configuration format | `{json.save.autoFormat: true}`                   |

The shorthand syntax supports the following features, described in more detail with examples below:

- Superset of JSON (valid JSON is valid shorthand)
- Optional commas, quotes, and sometimes colons
- Automatic type coercion
  - Support for bytes, dates, and maps with non-string keys
- Nested object & array creation
- Loading values from files
- Editing existing data
  - Appending & inserting to arrays
  - Unsetting properties
  - Moving properties & items
- Querying, array filtering, and field selection

The following are both completely valid shorthand and result in the same output:

```
{
  foo.bar[]{
    baz: 1
    hello: world
  }
}
```

```json
{
  "foo": {
    "bar": [
      {
        "baz": 1,
        "hello": "world"
      }
    ]
  }
}
```

This library has excellent test coverage and is additionally fuzz tested to ensure correctness and prevent panics.

## Alternatives & Inspiration

The CLI shorthand syntax is not the only one you can use to generate data for CLI commands. Here are some alternatives:

- [jo](https://github.com/jpmens/jo)
- [jarg](https://github.com/jdp/jarg)

For example, the shorthand example given above could be rewritten as:

```sh
$ jo -p foo=$(jo -p bar=$(jo -a $(jo -p baz=1 hello=world)))
```

The shorthand syntax implementation described herein uses those and the following for inspiration:

- [YAML](http://yaml.org/)
- [W3C HTML JSON Forms](https://www.w3.org/TR/html-json-forms/)
- [jq](https://stedolan.github.io/jq/)
- [JMESPath](http://jmespath.org/)

It seems reasonable to ask, why create a new syntax?

1. Built-in. No extra executables required. Your tool ships ready-to-go.
2. No need to use sub-shells to build complex structured data.
3. Syntax is closer to YAML & JSON and mimics how you do queries using tools like `jq` and `jmespath`.
4. It's _optional_, so you can use your favorite tool/language instead, while at the same time it provides a minimum feature set everyone will have in common.

## Features in Depth

You can use the included `j` executable to try out the shorthand format examples below. Examples are shown in JSON, but the shorthand parses into structured data that can be marshalled as other formats, like YAML or TOML if you prefer.

```sh
go get -u github.com/danielgtaylor/shorthand/cmd/j
```

Also feel free to use this tool to generate structured data for input to other commands.

### Keys & Values

At its most basic, a structure is built out of key & value pairs. They are separated by commas:

```sh
$ j hello: world, question: how are you?
{
  "hello": "world",
  "question": "how are you?"
}
```

### Types

Shorthand supports the standard JSON types, but adds some of its own as well to better support binary formats and its query features.

| Type      | Description                                                      |
| --------- | ---------------------------------------------------------------- |
| `null`    | JSON `null`                                                      |
| `boolean` | Either `true` or `false`                                         |
| `number`  | JSON number, e.g. `1`, `2.5`, or `1.4e5`                         |
| `string`  | Quoted or unquoted strings, e.g. `"hello"`                       |
| `bytes`   | `%`-prefixed, unquoted, base64-encoded binary data, e.g. `%wg==` |
| `time`    | Date/time in ISO8601, e.g. `2022-01-01T12:00:00Z`                |
| `array`   | JSON array, e.g. `[1, 2, 3]`                                     |
| `object`  | JSON object, e.g. `{"hello": "world"}`                           |

### Type Coercion

Well-known values like `null`, `true`, and `false` get converted to their respective types automatically. Numbers, bytes, and times also get converted. Similar to YAML, anything that doesn't fit one of those is treated as a string. This automatic coercion can be disabled by just wrapping your value in quotes.

```sh
# With coercion
$ j empty: null, bool: true, num: 1.5, string: hello
{
  "bool": true,
  "empty": null,
  "num": 1.5,
  "string": "hello"
}

# As strings
$ j empty: "null", bool: "true", num: "1.5", string: "hello"
{
  "bool": "true",
  "empty": "null",
  "num": "1.5",
  "string": "hello"
}

# Passing the empty string
$ j blank1: , blank2: ""
{
  "blank1": "",
  "blank2": ""
}
```

### Objects

Nested objects use a `.` separator when specifying the key.

```sh
$ j foo.bar.baz: 1
{
  "foo": {
    "bar": {
      "baz": 1
    }
  }
}
```

Properties of nested objects can be grouped by placing them inside `{` and `}`.

```sh
$ j foo.bar{id: 1, count.clicks: 5}
{
  "foo": {
    "bar": {
      "count": {
        "clicks": 5
      },
      "id": 1
    }
  }
}
```

### Arrays

Arrays are surrounded by square brackets like in JSON:

```sh
# Simple array
$ j [1, 2, 3]
[
  1,
  2,
  3
]
```

Array indexes use square brackets `[` and `]` to specify the zero-based index to set an item. If the index is out of bounds then `null` values are added as necessary to fill the array. Use an empty index `[]` to append to the an existing array. If the item is not an array, then a new one will be created.

```sh
# Nested arrays
$ j [0][2][0]: 1
[
  [
    null,
    null,
    [
      1
    ]
  ]
]

# Appending arrays
$ j a[]: 1, a[]: 2, a[]: 3
{
  "a": [
    1,
    2,
    3
  ]
}
```

### Loading from Files

Sometimes a field makes more sense to load from a file than to be specified on the commandline. The `@` preprocessor lets you load structured data, text, and bytes depending on the file extension and whether all bytes are valid UTF-8:

```sh
# Load a file's value as a parameter
$ j foo: @hello.txt
{
  "foo": "hello, world"
}

# Load structured data
$ j foo: @hello.json
{
  "foo": {
    "hello": "world"
  }
}
```

Remember, it's possible to disable this behavior with quotes:

```sh
$ j 'twitter: "@user"'
{
  "twitter": "@user"
}
```

### Patch (Partial Update)

Partial updates are supported on existing data, which can be used to implement HTTP `PATCH`, templating, and other similar features. This feature combines the best of both:

- [JSON Merge Patch](https://datatracker.ietf.org/doc/html/rfc7386)
- [JSON Patch](https://www.rfc-editor.org/rfc/rfc6902)

Partial updates support:

- Appending arrays via `[]`
- Inserting before via `[^index]`
- Removing fields or array items via `undefined`
- Moving/swapping fields or array items via `^`
  - The right hand side is a path to the value to swap

Some examples:

```sh
# First, let's create some data we'll modify later
$ j id: 1, tags: [a, b, c] >data.json

# Now let's append to the tags array
$ j <data.json 'tags[]: d'
{
  "id": 1,
  "tags": [
    "a",
    "b",
    "c",
    "d"
  ]
}

# Array item insertion
$ j <data.json 'tags[^0]: z'
{
  "id": 1,
  "tags": [
    "z",
    "a",
    "b",
    "c"
  ]
}

# Remove stuff
$ j <data.json 'id: undefined, tags[1]: undefined'
{
  "tags": [
    "a",
    "c"
  ]
}

# Rename the ID property, and swap the first/last array items
$ j <data.json 'id ^ name, tags[0] ^ tags[-1]'
{
  "name": 1,
  "tags": [
    "c",
    "b",
    "a"
  ]
}
```

### Querying

A data query language similar to the swap patch selection above is included, which allows you to query, filter, and select fields to return. This functionality is similar to, but simpler than, tools like:

- [jq](https://stedolan.github.io/jq/)
- [JMESPath](http://jmespath.org/)
- [JSON Path](https://www.ietf.org/archive/id/draft-ietf-jsonpath-base-06.html)

The query language supports:

- Paths for objects & arrays `foo.items.name`
- Array indexing `foo.items[1].name`
- Array filtering via [mexpr](https://github.com/danielgtaylor/mexpr) `foo.items[name.lower startsWith d]`
- Object property selection `foo.{created, names: items.name}`
- Stopping processing with a pipe `|`
- Flattening nested arrays `[]`

Examples:

```sh
# First, let's make a complex file to query
$ j 'users: [{id: 1, age: 5, friends: [a, b]}, {id: 2, age: 6, friends: [b, c]}, {id: 3, age: 5, friends: [c, d]}]' >data.json

# Query for each user's ID
$ j <data.json -q 'users.id'
[
  1,
  2,
  3
]

# Get the users who are friends with `b`
$ j <data.json -q 'users[friends contains "b"].id'
[
  1,
  2
]

# Get the ID & age of users who are friends with `b`
$ j <data.json -q 'users[friends contains "b"].{id, age}'
[
  {
    "age": null,
    "id": 1
  },
  {
    "age": null,
    "id": 2
  }
]
```

## Library Usage

The `GetInput` function provides an all-in-one quick and simple way to get input from both stdin and passed arguments:

```go
package main

import (
  "fmt"
  "github.com/danielgtaylor/shorthand"
)

func main() {
  result, err := shorthand.GetInput(os.Args[1:])
  if err != nil {
    panic(err)
  }

  fmt.Println(result)
}
```

It's also possible to get the shorthand representation of an input, for example:

```go
example := map[string]interface{}{
  "hello": "world",
  "labels": []interface{}{
    "one",
    "two",
  },
}

// Prints "hello: world, labels: [one, two]"
fmt.Println(shorthand.Get(example))
```

## Benchmarks

Shorthand v2 has been completely rewritten from the ground up and is over 20 times faster than v1, putting it at a similar speed/efficiency as the standard library's `encoding/json` package while supporting some compelling additional features:

```sh
$ go test -bench=.
goos: darwin
goarch: amd64
pkg: github.com/danielgtaylor/shorthand
cpu: Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz
BenchmarkMinJSON-12      	  534352	      2132 ns/op	    1808 B/op	      31 allocs/op
BenchmarkShorthandV2-12  	  309817	      3825 ns/op	    1888 B/op	      55 allocs/op
BenchmarkShorthandV1-12  	   14670	     83901 ns/op	   36436 B/op	     745 allocs/op
PASS
ok  	github.com/danielgtaylor/shorthand	4.750s
```

## Design & Implementation

The shorthand syntax is implemented as a [PEG](https://en.wikipedia.org/wiki/Parsing_expression_grammar) grammar which creates an AST-like object that is used to build an in-memory structure that can then be serialized out into formats like JSON, YAML, TOML, etc.

The `shorthand.peg` file implements the parser while the `shorthand.go` file implements the builder. Here's how you can test local changes to the grammar:

```sh
# One-time setup: install PEG compiler
$ go get -u github.com/mna/pigeon

# Make your shorthand.peg edits. Then:
$ go generate

# Next, rebuild the j executable.
$ go install ./cmd/j

# Now, try it out!
$ j <your-new-thing>
```
