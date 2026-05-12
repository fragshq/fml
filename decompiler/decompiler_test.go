package decompiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/theirish/fml/compiler"
	"github.com/theirish/fml/parser"
	"gopkg.in/yaml.v3"
)

func TestDecompiler_Basic(t *testing.T) {
	input := `system("Sys")
# The count
parameter("limit", type=int, default=10) # inline
set global = "hello"
session("s1", target="res", after="prev", expect=1==1, iterate=context.items) {
    use search
    use mcp kb
    call("tool") -> x {
        arg1 = "val"
        code( args.arg1.toUpper() )
    }
    context "ctx"
    - First line
      second line
    schema {
        field: string
    }
}
`
	// 1. Compile to YAML
	p, _ := parser.NewParser()
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	comp := compiler.New(plan)
	planYAML, err := comp.Compile()
	assert.NoError(t, err)

	// 2. Decompile back to FML
	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	// 3. Verify key constructs exist in decompiled output
	assert.Contains(t, output, `system("Sys")`)
	assert.Contains(t, output, `parameter("limit", type=int, default=10)`)
	assert.Contains(t, output, `set global = "hello"`)
	assert.Contains(t, output, `session("s1", target="res", after="prev", expect=1==1, iterate=context.items)`)
	assert.Contains(t, output, `use search`)
	assert.Contains(t, output, `use mcp kb`)
	assert.Contains(t, output, `call("tool") -> x`)
	assert.Contains(t, output, `- First line`)
	assert.Contains(t, output, `field: string`)

	// 4. Round-trip: Compile the decompiled output and compare with original YAML
	plan2, err := p.ParseString("roundtrip.frags", output)
	if err != nil {
		t.Fatalf("Failed to parse decompiled output: %v\nOutput was:\n%s", err, output)
	}
	comp2 := compiler.New(plan2)
	planYAML2, err := comp2.Compile()
	assert.NoError(t, err)

	// Compare YAML strings (ignoring comments for simplicity in comparison)
	y1, _ := yaml.Marshal(planYAML)
	y2, _ := yaml.Marshal(planYAML2)
	assert.Equal(t, string(y1), string(y2))
}

func TestDecompiler_NamespacedTargets(t *testing.T) {
	input := `session("s") {
    call("tool") -> context:var1 { }
    schema {
        result: string
    }
}
`
	p, _ := parser.NewParser()
	plan, _ := p.ParseString("test.frags", input)
	comp := compiler.New(plan)
	planYAML, _ := comp.Compile()

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	assert.Contains(t, output, `call("tool") -> context:var1`)

	// Round-trip
	plan2, err := p.ParseString("roundtrip.frags", output)
	assert.NoError(t, err)
	comp2 := compiler.New(plan2)
	planYAML2, _ := comp2.Compile()

	y1, _ := yaml.Marshal(planYAML)
	y2, _ := yaml.Marshal(planYAML2)
	assert.Equal(t, string(y1), string(y2))
}

func TestDecompiler_TransformerParser(t *testing.T) {
	input := `transformer("t1") {
    onFunctionOutput = "fn"
    jmesPath = "*"
    parser = json
}
`
	p, _ := parser.NewParser()
	plan, _ := p.ParseString("test.frags", input)
	comp := compiler.New(plan)
	planYAML, _ := comp.Compile()

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	assert.Contains(t, output, `parser = json`)

	// Round-trip
	plan2, err := p.ParseString("roundtrip.frags", output)
	assert.NoError(t, err)
	comp2 := compiler.New(plan2)
	planYAML2, _ := comp2.Compile()

	y1, _ := yaml.Marshal(planYAML)
	y2, _ := yaml.Marshal(planYAML2)
	assert.Equal(t, string(y1), string(y2))
}

func TestDecompiler_TransformerCode(t *testing.T) {
	input := `transformer("t1") {
    onFunctionOutput = "fn"
    jmesPath = "*"
    code( output.map(x => x.id) )
}
`
	p, _ := parser.NewParser()
	plan, _ := p.ParseString("test.frags", input)
	comp := compiler.New(plan)
	planYAML, _ := comp.Compile()

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	assert.Contains(t, output, `code( output.map(x => x.id) )`)

	// Round-trip
	plan2, err := p.ParseString("roundtrip.frags", output)
	assert.NoError(t, err)
	comp2 := compiler.New(plan2)
	planYAML2, _ := comp2.Compile()

	y1, _ := yaml.Marshal(planYAML)
	y2, _ := yaml.Marshal(planYAML2)
	assert.Equal(t, string(y1), string(y2))
}

func TestDecompiler_UnmarshalCompatibility(t *testing.T) {
	yamlInput := `
parameters:
  - name: p1
    schema: {type: string}
sessions:
  s1:
    prompt: hello
schema:
  type: object
  properties:
    s1:
      type: object
      x-session: s1
  required: [s1]
components:
  schemas:
    C1: {type: integer}
  prompts:
    base: "world"
`
	var plan compiler.PlanYAML
	err := yaml.Unmarshal([]byte(yamlInput), &plan)
	assert.NoError(t, err)

	dec := New(&plan)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	// Verify construction
	assert.Contains(t, output, `parameter("p1", type=string)`)
	assert.Contains(t, output, `session("s1")`)
	assert.Contains(t, output, `schema("C1")`)
	assert.Contains(t, output, `prompt("base")`)
}
