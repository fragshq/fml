package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/theirish/fml/parser"
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
	assert.Equal(t, "array", loopSchema.Type)
	assert.Equal(t, "object", loopSchema.Items.Type)
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
	out.Schema.Decode(&schema)
	assert.Equal(t, "array", schema.Properties["report"].Type)
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
	assert.Equal(t, "array", prop.Type)
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
	out.Transformers.Content[0].Decode(&t1)
	out.Transformers.Content[1].Decode(&t2)

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
	out.Transformers.Content[0].Decode(&t1)

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
	out.Transformers.Content[0].Decode(&t1)

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
	out.Parameters.Content[0].Decode(&p)

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
set x = 1

session("s") {
  # Description for f1
  schema {
    f1: string # inline f1
  }
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)

	// Check vars comment
	assert.Contains(t, out.Vars.Content[0].HeadComment, "Global var comment")

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
	preCallsNode.Content[0].Decode(&c1)
	preCallsNode.Content[1].Decode(&c2)
	preCallsNode.Content[2].Decode(&c3)

	assert.Equal(t, "vars", c1.In)
	assert.Equal(t, "var1", c1.Var)

	assert.Equal(t, "context", c2.In)
	assert.Equal(t, "var2", c2.Var)

	assert.Equal(t, "db", c3.In)
	assert.Equal(t, "var3", c3.Var)
}
