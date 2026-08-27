package decompiler

import (
	"strings"
	"testing"

	"github.com/fragshq/fml/compiler"
	"github.com/fragshq/fml/parser"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestDecompiler_ArraysAndDashes(t *testing.T) {
	input := `set tags = ["a", "b", "c"]
set my-config = {dashed-key: 1, "quoted key": 2}

session("s") {
    schema {
        "my field": string
    }
}
`
	p, _ := parser.NewParser()
	plan, _ := p.ParseString("test.frags", input)
	comp := compiler.New(plan)
	planYAML, _ := comp.Compile()

	decomp := New(planYAML)
	output, err := decomp.Decompile()
	assert.NoError(t, err)
	assert.Contains(t, output, `set tags = ["a", "b", "c"]`)
	assert.Contains(t, output, `dashed-key: 1`)
	assert.Contains(t, output, `"quoted key": 2`)
	assert.Contains(t, output, `"my field": string`)
}

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

func TestDecompiler_PromptWithBlankLines(t *testing.T) {
	input := `session("s") {
    - First
      
      Second
}
`
	p, _ := parser.NewParser()
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	comp := compiler.New(plan)
	planYAML, err := comp.Compile()
	assert.NoError(t, err)

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	// Round-trip
	plan2, err := p.ParseString("roundtrip.frags", output)
	if err != nil {
		t.Fatalf("Failed to parse output:\n%s\nError: %v", output, err)
	}
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

func TestDecompiler_UseWithAllowlist(t *testing.T) {
	input := `session("s") {
    use mcp tool1 {
        allowlist = ["m1", "m2"]
    }
}
`
	p, _ := parser.NewParser()
	plan, _ := p.ParseString("test.frags", input)
	comp := compiler.New(plan)
	planYAML, err := comp.Compile()
	assert.NoError(t, err)

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	assert.Contains(t, output, `use mcp tool1 {`)
	assert.Contains(t, output, `allowlist = ["m1", "m2"]`)

	// Round-trip
	plan2, err := p.ParseString("roundtrip.frags", output)
	assert.NoError(t, err)
	comp2 := compiler.New(plan2)
	planYAML2, _ := comp2.Compile()

	y1, _ := yaml.Marshal(planYAML)
	y2, _ := yaml.Marshal(planYAML2)
	assert.Equal(t, string(y1), string(y2))
}

func TestDecompiler_SchemaDescriptions(t *testing.T) {
	input := `session("s") {
    schema {
        name: string # The user's name
        age?: int # The user's age
    }
}

components {
    schema("User") {
        id: string # Unique identifier
    }
}
`
	p, _ := parser.NewParser()
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	comp := compiler.New(plan)
	planYAML, err := comp.Compile()
	assert.NoError(t, err)

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	assert.Contains(t, output, `name: string # The user's name`)
	assert.Contains(t, output, `age?: int # The user's age`)
	assert.Contains(t, output, `id: string # Unique identifier`)
}

func TestDecompiler_SystemMultiline(t *testing.T) {
	input := `system(` + "`" + `
  Line 1
  Line 2
` + "`" + `)

session("s") {
    - Prompt
}
`
	p, _ := parser.NewParser()
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	comp := compiler.New(plan)
	planYAML, err := comp.Compile()
	assert.NoError(t, err)

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	assert.Contains(t, output, "system(`")
	assert.Contains(t, output, "Line 1")
	assert.Contains(t, output, "Line 2")

	// Round-trip
	plan2, err := p.ParseString("roundtrip.frags", output)
	assert.NoError(t, err)
	comp2 := compiler.New(plan2)
	planYAML2, _ := comp2.Compile()

	y1, _ := yaml.Marshal(planYAML)
	y2, _ := yaml.Marshal(planYAML2)
	assert.Equal(t, string(y1), string(y2))
}

func TestDecompiler_PromptIndentation(t *testing.T) {
	input := `session("s") {
    - foo
      bar
        yay
}
`
	p, _ := parser.NewParser()
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	comp := compiler.New(plan)
	planYAML, err := comp.Compile()
	assert.NoError(t, err)

	// Check YAML value
	promptVal := ""
	for i := 0; i < len(planYAML.Sessions.Content[1].Content); i += 2 {
		if planYAML.Sessions.Content[1].Content[i].Value == "prompt" {
			promptVal = planYAML.Sessions.Content[1].Content[i+1].Value
		}
	}
	assert.Equal(t, "foo\nbar\n  yay", promptVal)

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	// Verify indentation in FML
	assert.Contains(t, output, "    - foo\n")
	assert.Contains(t, output, "      bar\n")
	assert.Contains(t, output, "        yay\n")

	// Round-trip
	plan2, err := p.ParseString("roundtrip.frags", output)
	assert.NoError(t, err)
	comp2 := compiler.New(plan2)
	planYAML2, _ := comp2.Compile()

	y1, _ := yaml.Marshal(planYAML)
	y2, _ := yaml.Marshal(planYAML2)
	assert.Equal(t, string(y1), string(y2))
}

func TestDecompiler_SchemaBlockDescription(t *testing.T) {
	input := `session("s") {
    schema {
        field: string
    } # Session level description
}
`
	p, _ := parser.NewParser()
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	comp := compiler.New(plan)
	planYAML, err := comp.Compile()
	assert.NoError(t, err)

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	assert.Contains(t, output, `} # Session level description`)
}

func TestDecompiler_ComponentSchemaDescription(t *testing.T) {
	input := `components {
    schema("User") {
        id: string
    } # User object description
}

session("s") {
    - Prompt
}
`
	p, _ := parser.NewParser()
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	comp := compiler.New(plan)
	planYAML, err := comp.Compile()
	assert.NoError(t, err)

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	assert.Contains(t, output, `} # User object description`)
}

func TestDecompiler_ComponentPromptDescription(t *testing.T) {
	input := `components {
    prompt("Base") {
        "You are a helpful assistant."
    } # System prompt base
}

session("s") {
    - Prompt
}
`
	p, _ := parser.NewParser()
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	comp := compiler.New(plan)
	planYAML, err := comp.Compile()
	assert.NoError(t, err)

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	assert.Contains(t, output, `} # System prompt base`)
}

func TestDecompiler_ParameterDescription(t *testing.T) {
	input := `parameter("p1", type=string) # The parameter description
`
	p, _ := parser.NewParser()
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	comp := compiler.New(plan)
	planYAML, err := comp.Compile()
	assert.NoError(t, err)

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	assert.Contains(t, output, `parameter("p1", type=string) # The parameter description`)
}

func TestDecompiler_ParameterDescriptionYAML(t *testing.T) {
	yamlInput := `
parameters:
  - name: p1
    schema:
      type: string
      description: "Manual description"
`
	var plan compiler.PlanYAML
	err := yaml.Unmarshal([]byte(yamlInput), &plan)
	assert.NoError(t, err)

	dec := New(&plan)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	assert.Contains(t, output, `parameter("p1", type=string) # Manual description`)
}

func TestDecompiler_ParameterEnum(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "String Enum",
			input:    `parameter("p", type=a|b|c)`,
			expected: `parameter("p", type=a|b|c)`,
		},
		{
			name:     "String Enum with explicit type",
			input:    `parameter("p", type=string, enum=a|b|c)`,
			expected: `parameter("p", type=a|b|c)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := parser.NewParser()
			plan, err := p.ParseString("test.frags", tt.input)
			assert.NoError(t, err)
			comp := compiler.New(plan)
			planYAML, err := comp.Compile()
			assert.NoError(t, err)

			dec := New(planYAML)
			output, err := dec.Decompile()
			assert.NoError(t, err)
			assert.Contains(t, output, tt.expected)
		})
	}
}

func TestDecompiler_ParameterComplexDescription(t *testing.T) {
	input := `parameter("config", type={
    url: string # The API URL
}) # Overall config
`
	p, _ := parser.NewParser()
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	comp := compiler.New(plan)
	planYAML, err := comp.Compile()
	assert.NoError(t, err)

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	assert.Contains(t, output, `parameter("config", type={`)
	assert.Contains(t, output, `url: string # The API URL`)
	assert.Contains(t, output, `}) # Overall config`)
}

func TestDecompiler_ParameterCapture(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected string
	}{
		{
			name: "Inline comment on parameter node",
			yaml: `
parameters:
  - name: p1 # Comment A
    schema: {type: string}
`,
			expected: `parameter("p1", type=string) # Comment A`,
		},
		{
			name: "Inline comment on schema node",
			yaml: `
parameters:
  - name: p2
    schema: {type: string} # Comment B
`,
			expected: `parameter("p2", type=string) # Comment B`,
		},
		{
			name: "Description field at parameter level",
			yaml: `
parameters:
  - name: p3
    description: "Description C"
    schema: {type: string}
`,
			expected: `parameter("p3", type=string) # Description C`,
		},
		{
			name: "Description field inside schema",
			yaml: `
parameters:
  - name: p4
    schema:
      type: string
      description: "Description D"
`,
			expected: `parameter("p4", type=string) # Description D`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var plan compiler.PlanYAML
			err := yaml.Unmarshal([]byte(tt.yaml), &plan)
			assert.NoError(t, err)

			dec := New(&plan)
			output, err := dec.Decompile()
			assert.NoError(t, err)

			assert.Contains(t, output, tt.expected)
		})
	}
}

func TestDecompiler_GenericComments(t *testing.T) {
	yamlInput := `
# Top level comment
systemPrompt: "Sys"
# Parameter block comment
parameters:
  - name: p1
    schema: {type: string}
# Call block comment
preCalls:
  - name: tool1
# Session block comment
sessions:
  # S1 comment
  s1:
    prompt: hello
`
	var plan compiler.PlanYAML
	err := yaml.Unmarshal([]byte(yamlInput), &plan)
	assert.NoError(t, err)

	dec := New(&plan)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	assert.Contains(t, output, "# Top level comment")
	assert.Contains(t, output, "# Parameter block comment")
	assert.Contains(t, output, "# Call block comment")
	assert.Contains(t, output, "# Session block comment")
	assert.Contains(t, output, "# S1 comment")
}

func TestDecompiler_GlobalPreCalls(t *testing.T) {
	input := `call("globalTool") -> res {
    arg1 = "val"
}

session("s") {
    - Prompt
}
`
	p, _ := parser.NewParser()
	plan, _ := p.ParseString("test.frags", input)
	comp := compiler.New(plan)
	planYAML, _ := comp.Compile()

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	assert.Contains(t, output, `call("globalTool") -> res`)

	// Round-trip
	plan2, err := p.ParseString("roundtrip.frags", output)
	assert.NoError(t, err)
	comp2 := compiler.New(plan2)
	planYAML2, err := comp2.Compile()
	assert.NoError(t, err)

	y1, _ := yaml.Marshal(planYAML)
	y2, _ := yaml.Marshal(planYAML2)
	assert.Equal(t, string(y1), string(y2))
}

func TestDecompiler_MixedSchemaPresence(t *testing.T) {
	input := `session("with_schema") {
    schema {
        f1: string
    }
}

session("without_schema") {
    - Just a prompt
}
`
	p, _ := parser.NewParser()
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	comp := compiler.New(plan)
	planYAML, err := comp.Compile()
	assert.NoError(t, err)

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	assert.Contains(t, output, `session("with_schema")`)
	assert.Contains(t, output, `f1: string`)
	assert.Contains(t, output, `session("without_schema")`)
	assert.NotContains(t, output, `without_schema: string`) // Should not contain default string schema
	assert.NotContains(t, output, `without_schema {`)

	// Ensure without_schema session does not have a schema block
	parts := strings.Split(output, `session("without_schema")`)
	assert.Len(t, parts, 2)
	assert.NotContains(t, parts[1], "schema")

	// Round-trip
	plan2, err := p.ParseString("roundtrip.frags", output)
	assert.NoError(t, err)
	comp2 := compiler.New(plan2)
	planYAML2, err := comp2.Compile()
	assert.NoError(t, err)

	y1, _ := yaml.Marshal(planYAML)
	y2, _ := yaml.Marshal(planYAML2)
	assert.Equal(t, string(y1), string(y2))
}

func TestDecompiler_BooleanTypes(t *testing.T) {
	input := `parameter("p1", type=bool)
parameter("p2", type=boolean)
session("s") {
    schema {
        f1: bool
        f2: boolean
    }
}
`
	p, _ := parser.NewParser()
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	comp := compiler.New(plan)
	planYAML, err := comp.Compile()
	assert.NoError(t, err)

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	// Since they both compile to JSON Schema "boolean", they should decompile to "bool"
	assert.Contains(t, output, `parameter("p1", type=bool)`)
	assert.Contains(t, output, `parameter("p2", type=bool)`)
	assert.Contains(t, output, `f1: bool`)
	assert.Contains(t, output, `f2: bool`)
}

func TestDecompiler_Annotations(t *testing.T) {
	input := `# @x-ui-layout = dashboard
# @x-ui-theme = dark

system("precise assistant")

# @x-ui-component = Input
# @x-ui-settings = {
#   placeholder = "Enter topic"
# }
# Regular description
parameter("topic", type=string)

components {
    # @x-ui-component = Card
    schema("CardData") {
        # @x-ui-component = Prose
        # @x-ui-settings = {
        #   columns = 2
        #   layout = "grid"
        #   order = [
        #     "kpi"
        #     "distribution"
        #   ]
        # }
        content: string
    }
}

session("gather") {
    # @x-ui-layout = grid
    schema {
        # @x-ui-hidden = true
        result: $CardData
    }
}
`
	p, _ := parser.NewParser()
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	comp := compiler.New(plan)
	planYAML, err := comp.Compile()
	assert.NoError(t, err)

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	// Verify output FML contains annotations and comments properly
	assert.Contains(t, output, `# @x-ui-layout = dashboard`)
	assert.Contains(t, output, `# @x-ui-theme = dark`)
	assert.Contains(t, output, `# @x-ui-component = Input`)
	assert.Contains(t, output, `#   placeholder = "Enter topic"`)
	assert.Contains(t, output, `# @x-ui-component = Card`)
	assert.Contains(t, output, `# @x-ui-settings = {`)
	assert.Contains(t, output, `#   columns = 2`)
	assert.Contains(t, output, `#   layout = grid`)
	assert.Contains(t, output, `#     kpi`)
	assert.Contains(t, output, `#     distribution`)
	assert.Contains(t, output, `# @x-ui-layout = grid`)
	assert.Contains(t, output, `# @x-ui-hidden = true`)
}

func TestDecompiler_StandardQualities(t *testing.T) {
	input := `session("validate") {
    schema {
        # @maximum = 100
        # @minimum = 18
        # @title = "User Age"
        age: int
        # @default = "user"
        # @enum = [
        #   "admin"
        #   "user"
        # ]
        # @pattern = "^[a-z]+$"
        role: string
    }
}
`
	p, _ := parser.NewParser()
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	comp := compiler.New(plan)
	planYAML, err := comp.Compile()
	assert.NoError(t, err)

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	// Verify standard qualities are preserved as annotations
	assert.Contains(t, output, `# @minimum = 18`)
	assert.Contains(t, output, `# @maximum = 100`)
	assert.Contains(t, output, `# @title = "User Age"`)
	assert.Contains(t, output, `# @default = user`)
	assert.Contains(t, output, `# @pattern = "^[a-z]+$"`)
	assert.Contains(t, output, `role: admin|user`)
}
