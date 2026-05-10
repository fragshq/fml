package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/theirish/fml/compiler"
	"github.com/theirish/fml/decompiler"
	"github.com/theirish/fml/parser"
	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "compile":
		runCompile(os.Args[2:])
	case "decompile":
		runDecompile(os.Args[2:])
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: fml <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  compile    Compile .fml to .yaml")
	fmt.Println("  decompile  Decompile .yaml to .fml")
	fmt.Println("  help       Show this help message")
	fmt.Println("\nUse 'fml <command> -help' for more information on a command.")
}

func runCompile(args []string) {
	fs := flag.NewFlagSet("compile", flag.ExitOnError)
	outputPath := fs.String("o", "", "Path to the output .yaml file (emits to stdout if omitted)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Println("Usage: fml compile <input.fml> [-o <output.yaml>]")
		os.Exit(1)
	}
	inputPath := fs.Arg(0)

	data, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Printf("Error reading input file: %v\n", err)
		os.Exit(1)
	}

	p, err := parser.NewParser()
	if err != nil {
		fmt.Printf("Error initializing parser: %v\n", err)
		os.Exit(1)
	}

	plan, err := p.ParseString(inputPath, string(data))
	if err != nil {
		fmt.Printf("Error parsing DSL: %v\n", err)
		os.Exit(1)
	}

	c := compiler.New(plan)
	out, err := c.Compile()
	if err != nil {
		fmt.Printf("Error during compilation: %v\n", err)
		os.Exit(1)
	}

	outYAML, err := yaml.Marshal(out)
	if err != nil {
		fmt.Printf("Error marshaling to YAML: %v\n", err)
		os.Exit(1)
	}

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

func runDecompile(args []string) {
	fs := flag.NewFlagSet("decompile", flag.ExitOnError)
	outputPath := fs.String("o", "", "Path to the output .fml file (emits to stdout if omitted)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Println("Usage: fml decompile <input.yaml> [-o <output.fml>]")
		os.Exit(1)
	}
	inputPath := fs.Arg(0)

	data, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Printf("Error reading input file: %v\n", err)
		os.Exit(1)
	}

	var planYAML compiler.PlanYAML
	err = yaml.Unmarshal(data, &planYAML)
	if err != nil {
		fmt.Printf("Error unmarshaling YAML: %v\n", err)
		os.Exit(1)
	}

	d := decompiler.New(&planYAML)
	out, err := d.Decompile()
	if err != nil {
		fmt.Printf("Error during decompilation: %v\n", err)
		os.Exit(1)
	}

	if *outputPath != "" {
		err = os.WriteFile(*outputPath, []byte(out), 0644)
		if err != nil {
			fmt.Printf("Error writing output file: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println(out)
	}
}
