package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/theirish/fml/compiler"
	"github.com/theirish/fml/decompiler"
	"github.com/theirish/fml/parser"
	"gopkg.in/yaml.v3"
)

var (
	outputPath string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "fml",
	Short: "FML is a Domain Specific Language for Frags LLM plans",
	Long: `FML (Frags Markup Language) is a compact, human-readable DSL 
that compiles to structured Frags YAML plans for LLM pipelines.`,
}

func init() {
	rootCmd.AddCommand(compileCmd)
	rootCmd.AddCommand(decompileCmd)

	compileCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Path to the output .yaml file (emits to stdout if omitted)")
	decompileCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Path to the output .fml file (emits to stdout if omitted)")
}

var compileCmd = &cobra.Command{
	Use:   "compile <input.fml>",
	Short: "Compile an FML file to a Frags YAML plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath := args[0]
		data, err := os.ReadFile(inputPath)
		if err != nil {
			return fmt.Errorf("error reading input file: %w", err)
		}

		p, err := parser.NewParser()
		if err != nil {
			return fmt.Errorf("error initializing parser: %w", err)
		}

		plan, err := p.ParseString(inputPath, string(data))
		if err != nil {
			return fmt.Errorf("error parsing DSL: %w", err)
		}

		c := compiler.New(plan)
		out, err := c.Compile()
		if err != nil {
			return fmt.Errorf("error during compilation: %w", err)
		}

		outYAML, err := yaml.Marshal(out)
		if err != nil {
			return fmt.Errorf("error marshaling to YAML: %w", err)
		}

		if outputPath != "" {
			if err := os.WriteFile(outputPath, outYAML, 0644); err != nil {
				return fmt.Errorf("error writing output file: %w", err)
			}
		} else {
			fmt.Println(string(outYAML))
		}
		return nil
	},
}

var decompileCmd = &cobra.Command{
	Use:   "decompile <input.yaml>",
	Short: "Decompile a Frags YAML plan back to FML source",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath := args[0]
		data, err := os.ReadFile(inputPath)
		if err != nil {
			return fmt.Errorf("error reading input file: %w", err)
		}

		var planYAML compiler.PlanYAML
		if err := yaml.Unmarshal(data, &planYAML); err != nil {
			return fmt.Errorf("error unmarshaling YAML: %w", err)
		}

		d := decompiler.New(&planYAML)
		out, err := d.Decompile()
		if err != nil {
			return fmt.Errorf("error during decompilation: %w", err)
		}

		if outputPath != "" {
			if err := os.WriteFile(outputPath, []byte(out), 0644); err != nil {
				return fmt.Errorf("error writing output file: %w", err)
			}
		} else {
			fmt.Println(out)
		}
		return nil
	},
}
