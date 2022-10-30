package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/danielgtaylor/shorthand/v2"
	"github.com/fxamacker/cbor/v2"
	toml "github.com/pelletier/go-toml"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"
)

func main() {
	var format *string
	var verbose *bool
	var query *string

	var debugLog func(string, ...any)

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [flags] key1: value1, key2: value2, ...", os.Args[0]),
		Short:   "Generate shorthand structured data",
		Example: fmt.Sprintf("%s foo.bar: 1, .baz: true", os.Args[0]),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 && *query == "" {
				fmt.Println("At least one arg or --query need to be passed")
				os.Exit(1)
			}
			if *verbose {
				debugLog = func(format string, a ...interface{}) {
					fmt.Printf(format, a...)
					fmt.Println()
				}
				fmt.Printf("Input: %s\n", strings.Join(args, " "))
			}
			result, err := shorthand.GetInputWithOptions(args, shorthand.ParseOptions{
				EnableFileInput:       true,
				EnableObjectDetection: true,
				ForceStringKeys:       *format == "json",
				DebugLogger:           debugLog,
			})
			if err != nil {
				if e, ok := err.(shorthand.Error); ok {
					fmt.Println(e.Pretty())
					os.Exit(1)
				} else {
					panic(err)
				}
			}

			if *query != "" {
				if selected, ok, err := shorthand.GetPath(*query, result, shorthand.GetOptions{DebugLogger: debugLog}); ok {
					result = selected
				} else if err != nil {
					fmt.Println(err.Pretty())
					os.Exit(1)
				} else {
					fmt.Println("No match")
					return
				}
			}

			var marshalled []byte

			switch *format {
			case "json":
				marshalled, err = json.MarshalIndent(result, "", "  ")
			case "cbor":
				marshalled, err = cbor.Marshal(result)
			case "yaml":
				marshalled, err = yaml.Marshal(result)
			case "toml":
				if k := reflect.TypeOf(result).Kind(); k != reflect.Map {
					err = fmt.Errorf("TOML only supports maps but found %s", k.String())
				} else {
					t, err := toml.TreeFromMap(result.(map[string]interface{}))
					if err == nil {
						marshalled = []byte(t.String())
					}
				}
			case "shorthand":
				marshalled = []byte(shorthand.MarshalPretty(result))
			}

			if err != nil {
				panic(err)
			}

			fmt.Println(string(marshalled))
		},
	}

	format = cmd.Flags().StringP("format", "f", "json", "Output format [json, cbor, yaml, toml, shorthand]")
	verbose = cmd.Flags().BoolP("verbose", "v", false, "Enable verbose output")
	query = cmd.Flags().StringP("query", "q", "", "Path to query")

	cmd.Execute()
}
