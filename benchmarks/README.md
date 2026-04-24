# Query Benchmarks

This module keeps comparison benchmark dependencies out of the root `go.mod`.

It compares shorthand query evaluation against Go implementations of:

- [JMESPath](https://github.com/jmespath/go-jmespath)
- [jq](https://github.com/itchyny/gojq)

The benchmark set covers:

- Simple field lookup
- Complex filter and projection
- Restish-style collection responses
- AWS CLI-style volume listing responses

Run the full suite with:

```sh
cd benchmarks
go test -run TestQueryEnginesMatchExpected -bench . -benchmem
```

The validation test makes sure the three query engines produce equivalent JSON
for each benchmark case before you compare timings.
