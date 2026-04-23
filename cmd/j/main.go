package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/danielgtaylor/shorthand/v2"
	"github.com/fxamacker/cbor/v2"
	toml "github.com/pelletier/go-toml"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"
)

type stdinStatter interface {
	Stat() (os.FileInfo, error)
}

func isStdinPiped(stdin stdinStatter) (bool, error) {
	stat, err := stdin.Stat()
	if err != nil {
		return false, err
	}
	return (stat.Mode() & os.ModeCharDevice) == 0, nil
}

func marshalOutput(result any, format string) ([]byte, error) {
	switch format {
	case "json":
		return json.MarshalIndent(result, "", "  ")
	case "cbor":
		return cbor.Marshal(result)
	case "yaml":
		return yaml.Marshal(result)
	case "toml":
		if result == nil {
			return nil, fmt.Errorf("TOML only supports maps but found null")
		}
		converted := shorthand.ConvertMapString(result)
		m, ok := converted.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("TOML only supports maps but found %T", result)
		}
		t, err := toml.TreeFromMap(m)
		if err != nil {
			return nil, err
		}
		return []byte(t.String()), nil
	case "shorthand":
		return []byte(shorthand.MarshalPretty(result)), nil
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}

func main() {
	var format *string
	var verbose *bool
	var query *string

	var debugLog func(string, ...any)

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [flags] key1: value1, key2: value2, ...", os.Args[0]),
		Short:   "Generate shorthand structured data",
		Example: fmt.Sprintf("%s foo{bar: 1, baz: true}", os.Args[0]),
		Run: func(cmd *cobra.Command, args []string) {
			stdinPiped, err := isStdinPiped(os.Stdin)
			if err != nil {
				fmt.Printf("Unable to inspect stdin: %v\n", err)
				os.Exit(1)
			}
			if len(args) == 0 && *query == "" && !stdinPiped {
				fmt.Println("At least one arg or --query need to be passed")
				os.Exit(1)
			}
			if *verbose {
				debugLog = func(format string, a ...any) {
					fmt.Printf(format, a...)
					fmt.Println()
				}
				fmt.Printf("Input: %s\n", strings.Join(args, " "))
			}
			result, isStructured, err := shorthand.GetInput(args, shorthand.ParseOptions{
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
					fmt.Println(err)
					os.Exit(1)
				}
			}
			if !isStructured {
				fmt.Println("Input file could not be parsed as structured data")
				os.Exit(1)
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

			marshalled, err := marshalOutput(result, *format)

			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			fmt.Println(string(marshalled))
		},
	}

	format = cmd.Flags().StringP("format", "f", "json", "Output format [json, cbor, yaml, toml, shorthand]")
	verbose = cmd.Flags().BoolP("verbose", "v", false, "Enable verbose output")
	query = cmd.Flags().StringP("query", "q", "", "Path to query")

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
