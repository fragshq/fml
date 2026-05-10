package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/theirish/fml/compiler"
	"github.com/theirish/fml/parser"
	"gopkg.in/yaml.v3"
)

// main is the CLI entry point for the Frags DSL compiler.
func main() {
	inputPath := flag.String("i", "", "Path to the input .frags DSL file")
	outputPath := flag.String("o", "", "Path to the output .yaml file (emits to stdout if omitted)")
	flag.Parse()

	if *inputPath == "" {
		fmt.Println("Usage: frags-compiler -i <input.frags> [-o <output.yaml>]")
		os.Exit(1)
	}

	// 1. Read source DSL
	data, err := os.ReadFile(*inputPath)
	if err != nil {
		fmt.Printf("Error reading input file: %v\n", err)
		os.Exit(1)
	}

	// 2. Initialize Parser
	p, err := parser.NewParser()
	if err != nil {
		fmt.Printf("Error initializing parser: %v\n", err)
		os.Exit(1)
	}

	// 3. Parse DSL into AST
	plan, err := p.ParseString(*inputPath, string(data))
	if err != nil {
		fmt.Printf("Error parsing DSL: %v\n", err)
		os.Exit(1)
	}

	// 4. Compile AST into Plan structure
	c := compiler.New(plan)
	out, err := c.Compile()
	if err != nil {
		fmt.Printf("Error during compilation: %v\n", err)
		os.Exit(1)
	}

	// 5. Marshal to YAML
	outYAML, err := yaml.Marshal(out)
	if err != nil {
		fmt.Printf("Error marshaling to YAML: %v\n", err)
		os.Exit(1)
	}

	// 6. Emit result
	if *outputPath != "" {
		err = os.WriteFile(*outputPath, outYAML, 0644)
		if err != nil {
			fmt.Printf("Error writing output file: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println(string(outYAML))
	}
}
