package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/danielgtaylor/shorthand"
	toml "github.com/pelletier/go-toml"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"
)

func main() {
	var format *string

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [flags] key1: value1, key2: value2, ...", os.Args[0]),
		Short:   "Generate shorthand structured data",
		Example: fmt.Sprintf("%s foo.bar: 1, .baz: true", os.Args[0]),
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			result, err := shorthand.GetInput(args)
			if err != nil {
				panic(err)
			}

			var marshalled []byte

			switch *format {
			case "json":
				marshalled, err = json.MarshalIndent(result, "", "  ")
			case "yaml":
				marshalled, err = yaml.Marshal(result)
			case "toml":
				t, err := toml.TreeFromMap(result)
				if err == nil {
					marshalled = []byte(t.String())
				}
			case "shorthand":
				marshalled = []byte(shorthand.Get(result))
			}

			if err != nil {
				panic(err)
			}

			fmt.Println(string(marshalled))
		},
	}

	format = cmd.Flags().StringP("format", "f", "json", "Output format [json, yaml, toml, shorthand]")

	cmd.Execute()
}
