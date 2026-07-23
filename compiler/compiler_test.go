package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/theirish81/fml/parser"
	"gopkg.in/yaml.v3"
)

func compileSource(t *testing.T, input string) (*PlanYAML, error) {
	p, err := parser.NewParser()
	if err != nil {
		t.Fatalf("failed to create parser: %v", err)
	}
	plan, err := p.ParseString("test.frags", input)
	if err != nil {
		return nil, err
	}
	c := New(plan)
	return c.Compile()
}

func TestCompiler_SetArray(t *testing.T) {
	input := `set tags = ["a", "b", "c"]`
	out, err := compileSource(t, input)
	assert.NoError(t, err)
	assert.NotNil(t, out.Vars)

	// Vars is a MappingNode: [keyNode, valNode]
	assert.Equal(t, "tags", out.Vars.Content[0].Value)
	valNode := out.Vars.Content[1]

	var tags []string
	err = valNode.Decode(&tags)
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, tags)
}

func TestCompiler_DashAndQuotedKeys(t *testing.T) {
	input := `
set my-config = { 
  "quoted-key": 1,
  dashed-key: 2
}
session("s") {
  schema {
    "my-field": string
  }
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)

	// Check vars
	var config map[string]int
	err = out.Vars.Content[1].Decode(&config)
	assert.NoError(t, err)
	assert.Equal(t, 1, config["quoted-key"])
	assert.Equal(t, 2, config["dashed-key"])

	// Check schema
	var schema JSONSchema
	err = out.Schema.Decode(&schema)
	assert.NoError(t, err)
	assert.Contains(t, schema.Properties["s"].Properties, "my-field")
}

func TestCompiler_ParameterDefaultArray(t *testing.T) {
	input := `parameter("tags", type=string[], default=["a", "b"])`
	out, err := compileSource(t, input)
	assert.NoError(t, err)
	require.Len(t, out.Parameters.Content, 1)

	type Param struct {
		Name   string     `yaml:"name"`
		Schema JSONSchema `yaml:"schema"`
	}
	var p Param
	err = out.Parameters.Content[0].Decode(&p)
	assert.NoError(t, err)
	assert.Equal(t, "tags", p.Name)
	assert.Equal(t, []interface{}{"a", "b"}, p.Schema.Default)
}

func TestCompiler_ParameterEnum(t *testing.T) {
	input := `parameter("p", type=string, enum=a|b|c)`
	out, err := compileSource(t, input)
	assert.NoError(t, err)
	require.Len(t, out.Parameters.Content, 1)

	var p struct {
		Name   string     `yaml:"name"`
		Schema JSONSchema `yaml:"schema"`
	}
	err = out.Parameters.Content[0].Decode(&p)
	assert.NoError(t, err)
	assert.Equal(t, "p", p.Name)
	assert.Equal(t, "string", p.Schema.Type)
	assert.Equal(t, []interface{}{"a", "b", "c"}, p.Schema.Enum)
}

func TestCompiler_ParameterEnumConflict(t *testing.T) {
	input := `parameter("p", type=int, enum=a|b|c)`
	_, err := compileSource(t, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "enum can only be used with string type")
}

func TestCompiler_SessionGrouping(t *testing.T) {
	input := `
session("s1") { schema { f1: string } }
session("s2") { schema { f2: int } }
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)
	assert.NotNil(t, out.Schema)

	var schema JSONSchema
	err = out.Schema.Decode(&schema)
	assert.NoError(t, err)

	assert.Contains(t, schema.Properties, "s1")
	assert.Contains(t, schema.Properties, "s2")
}

func TestCompiler_SessionNoSchema(t *testing.T) {
	input := `
session("s1") { 
    - prompt 1
}
session("s2") { 
    schema { f2: int } 
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)
	assert.NotNil(t, out.Schema)

	var schema JSONSchema
	err = out.Schema.Decode(&schema)
	assert.NoError(t, err)

	// s1 should NOT be in the root schema because it has no schema block
	assert.NotContains(t, schema.Properties, "s1")
	assert.Contains(t, schema.Properties, "s2")

	// s1 should NOT be in required
	assert.NotContains(t, schema.Required, "s1")
	assert.Contains(t, schema.Required, "s2")
}

func TestCompiler_Iteration(t *testing.T) {
	input := `
session("loop", iterate=context.items) {
  schema {
    item: string
  }
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)

	var schema JSONSchema
	err = out.Schema.Decode(&schema)
	assert.NoError(t, err)

	loopSchema := schema.Properties["loop"]
	assert.Equal(t, parser.TypeArray, loopSchema.Type)
	assert.Equal(t, parser.TypeObject, loopSchema.Items.Type)
}

func TestCompiler_ValidationErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		err   string
	}{
		{
			"FieldCollisionAcrossBlocks",
			`session("s1") { schema { f1: string } } session("s1") { schema { f1: int } }`,
			"field \"f1\" already defined",
		},
		{
			"AnonymousFieldConflictInSameSession",
			`session("s1") { schema string[] schema { f1: int } }`,
			"has both anonymous schema and field schema",
		},
		{
			"DuplicateSessionField",
			`session("s1") { schema { f1: string f1: int } }`,
			"field \"f1\" already defined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileSource(t, tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.err)
		})
	}
}

func TestCompiler_ComplexPlan(t *testing.T) {
	input := `
system("Research Assistant")

parameter("topic", type=string)
parameter("max_results", type=int, default=5)

transformer("filter") {
    onFunctionOutput = "search"
    jmesPath = "[*].id"
}

session("gather") {
    use search
    call("search") -> results {
        query = "{{ .params.topic }}"
    }
    - Found results for {{ .params.topic }}
    schema {
        ids: string[]
    }
}

session("report", after="gather", iterate=gather.ids) {
    - Elaborating on {{ .it }}
    schema {
        details: string
    }
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)
	assert.NotNil(t, out)

	// Verify session ordering
	assert.Equal(t, []string{"gather", "report"}, cOrder(out))

	// Verify iterate wrapping
	var schema JSONSchema
	_ = out.Schema.Decode(&schema)
	assert.Equal(t, parser.TypeArray, schema.Properties["report"].Type)
}

func TestCompiler_TargetRenaming(t *testing.T) {
	input := `
session("gather", target="results") {
  schema {
    found: bool
  }
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)

	var schema JSONSchema
	err = out.Schema.Decode(&schema)
	assert.NoError(t, err)

	assert.Contains(t, schema.Properties, "results")
	assert.NotContains(t, schema.Properties, "gather")
	assert.Equal(t, "gather", schema.Properties["results"].XSession)
	assert.Contains(t, schema.Required, "results")
}

func TestCompiler_TargetWithIteration(t *testing.T) {
	input := `
session("gather") { schema { ids: int[] } }
session("process", after="gather", iterate=gather.ids, target="processed_items") {
  schema {
    result: string
  }
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)

	var schema JSONSchema
	err = out.Schema.Decode(&schema)
	assert.NoError(t, err)

	// Check that 'process' was renamed to 'processed_items'
	assert.Contains(t, schema.Properties, "processed_items")
	assert.NotContains(t, schema.Properties, "process")

	// Verify it is still an array because of 'iterate'
	prop := schema.Properties["processed_items"]
	assert.Equal(t, parser.TypeArray, prop.Type)
	assert.Equal(t, "process", prop.XSession)
}

func TestCompiler_TransformerParser(t *testing.T) {
	input := `
transformer("t1") {
    onFunctionOutput = "fn"
    jmesPath = "*"
    parser = "json"
}
transformer("t2") {
    onFunctionOutput = "fn2"
    jmesPath = "*"
    parser = "csv"
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)
	assert.NotNil(t, out.Transformers)

	type Trans struct {
		Name   string `yaml:"name"`
		Parser string `yaml:"parser"`
	}
	var t1, t2 Trans
	_ = out.Transformers.Content[0].Decode(&t1)
	_ = out.Transformers.Content[1].Decode(&t2)

	assert.Equal(t, "t1", t1.Name)
	assert.Equal(t, "json", t1.Parser)
	assert.Equal(t, "t2", t2.Name)
	assert.Equal(t, "csv", t2.Parser)
}

func TestCompiler_TransformerCode(t *testing.T) {
	input := `
transformer("t1") {
    onFunctionOutput = "fn"
    jmesPath = "*"
    code( output.map(x => x.id) )
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)
	assert.NotNil(t, out.Transformers)

	type Trans struct {
		Name string `yaml:"name"`
		Code string `yaml:"code"`
	}
	var t1 Trans
	_ = out.Transformers.Content[0].Decode(&t1)

	assert.Equal(t, "t1", t1.Name)
	assert.Equal(t, "output.map(x => x.id)", t1.Code)
}

func TestCompiler_TransformerParserUnquoted(t *testing.T) {
	input := `
transformer("t1") {
    onFunctionOutput = "fn"
    jmesPath = "*"
    parser = json
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)
	assert.NotNil(t, out.Transformers)

	type Trans struct {
		Name   string `yaml:"name"`
		Parser string `yaml:"parser"`
	}
	var t1 Trans
	_ = out.Transformers.Content[0].Decode(&t1)

	assert.Equal(t, "t1", t1.Name)
	assert.Equal(t, "json", t1.Parser)
}

func TestCompiler_NestedSchemaComments(t *testing.T) {
	input := `
session("s") {
  schema {
    user: {
      name: string # The user's name
      age: int # The user's age
    }
  }
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)

	var schema JSONSchema
	err = out.Schema.Decode(&schema)
	assert.NoError(t, err)

	userSchema := schema.Properties["s"].Properties["user"]
	assert.Equal(t, "The user's name", userSchema.Properties["name"].Description)
	assert.Equal(t, "The user's age", userSchema.Properties["age"].Description)
}

func TestCompiler_NestedParameterComments(t *testing.T) {
	input := `
parameter("config", type={
    retries: int # Max retries
    timeout: float # Request timeout
})
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)

	// parameters is a SequenceNode
	require.Equal(t, 1, len(out.Parameters.Content))

	type Param struct {
		Name   string     `yaml:"name"`
		Schema JSONSchema `yaml:"schema"`
	}
	var p Param
	_ = out.Parameters.Content[0].Decode(&p)

	configSchema := p.Schema
	assert.Equal(t, "Max retries", configSchema.Properties["retries"].Description)
	assert.Equal(t, "Request timeout", configSchema.Properties["timeout"].Description)
}

func cOrder(p *PlanYAML) []string {
	var order []string
	// sessions is a MappingNode, content is [key, val, key, val...]
	for i := 0; i < len(p.Sessions.Content); i += 2 {
		order = append(order, p.Sessions.Content[i].Value)
	}
	return order
}

func TestCompiler_Comments(t *testing.T) {
	input := `
# Global var comment
# Another global comment line
set x = 1

session("s") {
  schema {
    # Description line 1
    # Description line 2
    f1: string # inline f1
  }
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)

	// Check vars comment
	assert.Contains(t, out.Vars.Content[0].HeadComment, "Global var comment\nAnother global comment line")

	var schema JSONSchema
	err = out.Schema.Decode(&schema)
	assert.NoError(t, err)

	sSchema := schema.Properties["s"]
	f1Schema := sSchema.Properties["f1"]
	assert.Equal(t, "inline f1", f1Schema.Description)
}

func TestCompiler_CallOutputNamespaces(t *testing.T) {
	input := `
session("s") {
  call("tool1") -> var1 { }
  call("tool2") -> context:var2 { }
  call("tool3") -> db:var3 { }
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)

	// Get the session 's'
	sessNode := out.Sessions.Content[1]
	var preCallsNode *yaml.Node
	for i := 0; i < len(sessNode.Content); i += 2 {
		if sessNode.Content[i].Value == "preCalls" {
			preCallsNode = sessNode.Content[i+1]
			break
		}
	}
	require.NotNil(t, preCallsNode)
	assert.Equal(t, 3, len(preCallsNode.Content))

	type Call struct {
		In  string `yaml:"in"`
		Var string `yaml:"var"`
	}

	var c1, c2, c3 Call
	_ = preCallsNode.Content[0].Decode(&c1)
	_ = preCallsNode.Content[1].Decode(&c2)
	_ = preCallsNode.Content[2].Decode(&c3)

	assert.Equal(t, "vars", c1.In)
	assert.Equal(t, "var1", c1.Var)

	assert.Equal(t, "context", c2.In)
	assert.Equal(t, "var2", c2.Var)

	assert.Equal(t, "db", c3.In)
	assert.Equal(t, "var3", c3.Var)
}

func TestCompiler_Prompts(t *testing.T) {
	input := `
session("s") {
  + pre1
  + pre2
  - prompt
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)

	sessNode := out.Sessions.Content[1]
	var prePrompt, prompt *yaml.Node
	for i := 0; i < len(sessNode.Content); i += 2 {
		if sessNode.Content[i].Value == "prePrompt" {
			prePrompt = sessNode.Content[i+1]
		} else if sessNode.Content[i].Value == "prompt" {
			prompt = sessNode.Content[i+1]
		}
	}

	require.NotNil(t, prePrompt)
	assert.Equal(t, yaml.SequenceNode, prePrompt.Kind)
	assert.Len(t, prePrompt.Content, 2)
	assert.Equal(t, "pre1", prePrompt.Content[0].Value)
	assert.Equal(t, "pre2", prePrompt.Content[1].Value)

	require.NotNil(t, prompt)
	assert.Equal(t, "prompt", prompt.Value)
}

func TestCompiler_UseWithAllowlist(t *testing.T) {
	input := `session("s") { 
  use mcp tool1 { 
    allowlist = ["m1", "m2"] 
  }
}`
	out, err := compileSource(t, input)
	assert.NoError(t, err)

	// sessions is MappingNode, index 0 is "s", index 1 is sessNode
	sessNode := out.Sessions.Content[1]
	var toolsNode *yaml.Node
	for i := 0; i < len(sessNode.Content); i += 2 {
		if sessNode.Content[i].Value == "tools" {
			toolsNode = sessNode.Content[i+1]
			break
		}
	}
	require.NotNil(t, toolsNode)
	assert.Equal(t, 1, len(toolsNode.Content))

	var tool ToolYAML
	err = toolsNode.Content[0].Decode(&tool)
	assert.NoError(t, err)
	assert.Equal(t, "mcp", tool.Type)
	assert.Equal(t, "tool1", tool.Name)
	assert.Equal(t, []string{"m1", "m2"}, tool.Allowlist)
}

func TestCompiler_Resources(t *testing.T) {
	input := `
session("s") {
  resource "data.csv"
  resource "prompt.txt" -> vars:template
  resource "config.json" -> myConfig
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)

	sessNode := out.Sessions.Content[1]
	var resNode *yaml.Node
	for i := 0; i < len(sessNode.Content); i += 2 {
		if sessNode.Content[i].Value == "resources" {
			resNode = sessNode.Content[i+1]
			break
		}
	}
	require.NotNil(t, resNode)
	assert.Equal(t, 3, len(resNode.Content))

	type Res struct {
		Identifier string `yaml:"identifier"`
		In         string `yaml:"in"`
		Var        string `yaml:"var"`
	}

	var r1, r2, r3 Res
	_ = resNode.Content[0].Decode(&r1)
	_ = resNode.Content[1].Decode(&r2)
	_ = resNode.Content[2].Decode(&r3)

	assert.Equal(t, "data.csv", r1.Identifier)
	assert.Empty(t, r1.In)
	assert.Empty(t, r1.Var)

	assert.Equal(t, "prompt.txt", r2.Identifier)
	assert.Equal(t, "vars", r2.In)
	assert.Equal(t, "template", r2.Var)

	assert.Equal(t, "config.json", r3.Identifier)
	assert.Equal(t, "vars", r3.In)
	assert.Equal(t, "myConfig", r3.Var)
}

func TestCompiler_MultilinePromptIndentation(t *testing.T) {
	input := `
session("s") {
  - foo
    bar
      yay
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)

	sessNode := out.Sessions.Content[1]
	var prompt *yaml.Node
	for i := 0; i < len(sessNode.Content); i += 2 {
		if sessNode.Content[i].Value == "prompt" {
			prompt = sessNode.Content[i+1]
		}
	}

	require.NotNil(t, prompt)
	// Expected: "foo\nbar\n  yay"
	// Wait, let's trace:
	// "- foo" -> line starts with "-" at column 2. indent=2.
	// text after "- " is "foo"
	// "    bar" -> column 4. stripLen = indent + 1 = 3. line[3:] = " bar"
	// "      yay" -> column 6. line[3:] = "   yay"
	// So current logic produces "foo\n bar\n   yay"?
	// Let's see what the user says: "+ foo\n   bar\n    yay" -> "foo\nbar\nyay" (lost)
	assert.Equal(t, "foo\nbar\n  yay", prompt.Value)
}

func TestCompiler_PromptWithBlankLine(t *testing.T) {
	input := `
session("s") {
  - First line
    
    Second line
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)

	sessNode := out.Sessions.Content[1]
	var prompt *yaml.Node
	for i := 0; i < len(sessNode.Content); i += 2 {
		if sessNode.Content[i].Value == "prompt" {
			prompt = sessNode.Content[i+1]
		}
	}

	require.NotNil(t, prompt)
	// Currently this might fail or produce only "First line"
	assert.Contains(t, prompt.Value, "First line")
	assert.Contains(t, prompt.Value, "Second line")
}

func TestCompiler_PromptWithTrulyBlankLine(t *testing.T) {
	input := "session(\"s\") {\n  - First\n\n    Second\n}\n"
	out, err := compileSource(t, input)
	assert.NoError(t, err)

	sessNode := out.Sessions.Content[1]
	var prompt *yaml.Node
	for i := 0; i < len(sessNode.Content); i += 2 {
		if sessNode.Content[i].Value == "prompt" {
			prompt = sessNode.Content[i+1]
		}
	}

	require.NotNil(t, prompt)
	assert.Equal(t, "First\n\nSecond", prompt.Value)
}

func TestCompiler_BooleanTypes(t *testing.T) {
	input := `
parameter("paramBool", type=bool)
parameter("paramBoolean", type=boolean)
session("gather") {
  schema {
    f1: bool
    f2: boolean
  }
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)
	assert.NotNil(t, out)

	// Check parameter compilation
	type Param struct {
		Name   string     `yaml:"name"`
		Schema JSONSchema `yaml:"schema"`
	}
	require.Len(t, out.Parameters.Content, 2)

	var p1, p2 Param
	err = out.Parameters.Content[0].Decode(&p1)
	assert.NoError(t, err)
	assert.Equal(t, "paramBool", p1.Name)
	assert.Equal(t, "boolean", p1.Schema.Type)

	err = out.Parameters.Content[1].Decode(&p2)
	assert.NoError(t, err)
	assert.Equal(t, "paramBoolean", p2.Name)
	assert.Equal(t, "boolean", p2.Schema.Type)

	// Check schema compilation
	var schema JSONSchema
	err = out.Schema.Decode(&schema)
	assert.NoError(t, err)

	gatherSchema := schema.Properties["gather"]
	require.NotNil(t, gatherSchema)
	assert.Equal(t, "object", gatherSchema.Type)
	assert.Equal(t, "boolean", gatherSchema.Properties["f1"].Type)
	assert.Equal(t, "boolean", gatherSchema.Properties["f2"].Type)
}

func TestCompiler_Annotations(t *testing.T) {
	input := `
# @x-ui-layout = dashboard
# @x-ui-theme = dark

system("Precise Assistant")

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
        #   layout = "grid"
        #   columns = 2
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
	out, err := compileSource(t, input)
	assert.NoError(t, err)
	assert.NotNil(t, out)

	// Verify parameter annotations
	type Param struct {
		Name   string     `yaml:"name"`
		Schema JSONSchema `yaml:"schema"`
	}
	require.Len(t, out.Parameters.Content, 1)
	var p Param
	err = out.Parameters.Content[0].Decode(&p)
	assert.NoError(t, err)
	assert.Equal(t, "topic", p.Name)
	assert.Equal(t, "Regular description", p.Schema.Description)
	assert.Equal(t, "Input", p.Schema.Extensions["x-ui-component"])
	settings, ok := p.Schema.Extensions["x-ui-settings"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Enter topic", settings["placeholder"])

	// Verify component schema annotations
	cardSchema := out.Components.Schemas["CardData"]
	require.NotNil(t, cardSchema)
	assert.Equal(t, "Card", cardSchema.Extensions["x-ui-component"])
	contentField := cardSchema.Properties["content"]
	require.NotNil(t, contentField)
	assert.Equal(t, "Prose", contentField.Extensions["x-ui-component"])
	contentSettings, ok := contentField.Extensions["x-ui-settings"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "grid", contentSettings["layout"])
	assert.Equal(t, 2, contentSettings["columns"])
	order, ok := contentSettings["order"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, []interface{}{"kpi", "distribution"}, order)

	// Verify session schema annotations
	var schema JSONSchema
	err = out.Schema.Decode(&schema)
	assert.NoError(t, err)
	assert.Equal(t, "dashboard", schema.Extensions["x-ui-layout"])
	assert.Equal(t, "dark", schema.Extensions["x-ui-theme"])
	gatherSchema := schema.Properties["gather"]
	require.NotNil(t, gatherSchema)
	assert.Equal(t, "grid", gatherSchema.Extensions["x-ui-layout"])
	resultField := gatherSchema.Properties["result"]
	require.NotNil(t, resultField)
	assert.Equal(t, true, resultField.Extensions["x-ui-hidden"])
}
