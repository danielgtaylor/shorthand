package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

func commandName(args0 string) string {
	if base := filepath.Base(args0); base != "" && base != "." && base != string(filepath.Separator) {
		return base
	}
	return "j"
}

func main() {
	var format *string
	var verbose *bool
	var query *string

	var debugLog func(string, ...any)
	name := commandName(os.Args[0])

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [flags] key1: value1, key2: value2, ...", name),
		Short:   "Generate shorthand structured data",
		Example: fmt.Sprintf("%s foo{bar: 1, baz: true}", name),
		Run: func(cmd *cobra.Command, args []string) {
			stdinPiped, err := isStdinPiped(os.Stdin)
			if err != nil {
				cmd.PrintErrf("Unable to inspect stdin: %v\n", err)
				os.Exit(1)
			}
			if len(args) == 0 && *query == "" && !stdinPiped {
				cmd.PrintErrln("At least one arg or --query must be provided")
				os.Exit(1)
			}
			if *verbose {
				debugLog = func(format string, a ...any) {
					cmd.PrintErrf(format, a...)
					cmd.PrintErrln()
				}
				cmd.PrintErrf("Input: %s\n", strings.Join(args, " "))
			}
			result, isStructured, err := shorthand.GetInput(args, shorthand.ParseOptions{
				EnableFileInput:       true,
				EnableObjectDetection: true,
				ForceStringKeys:       *format == "json",
				DebugLogger:           debugLog,
			})
			if err != nil {
				if e, ok := err.(shorthand.Error); ok {
					cmd.PrintErrln(e.Pretty())
					os.Exit(1)
				} else {
					cmd.PrintErrln(err)
					os.Exit(1)
				}
			}
			if !isStructured {
				cmd.PrintErrln("Input file could not be parsed as structured data")
				os.Exit(1)
			}

			if *query != "" {
				if selected, ok, err := shorthand.GetPath(*query, result, shorthand.GetOptions{DebugLogger: debugLog}); ok {
					result = selected
				} else if err != nil {
					cmd.PrintErrln(err.Pretty())
					os.Exit(1)
				} else {
					cmd.PrintErrln("No match")
					return
				}
			}

			marshalled, err := marshalOutput(result, *format)

			if err != nil {
				cmd.PrintErrln(err)
				os.Exit(1)
			}

			fmt.Println(string(marshalled))
		},
	}

	format = cmd.Flags().StringP("format", "f", "json", "Output format [json, cbor, yaml, toml, shorthand]")
	verbose = cmd.Flags().BoolP("verbose", "v", false, "Enable verbose output")
	query = cmd.Flags().StringP("query", "q", "", "Path to query")

	if err := cmd.Execute(); err != nil {
		cmd.PrintErrln(err)
		os.Exit(1)
	}
}
