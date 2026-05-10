package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/theirish/fml/parser"
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
			`session("s1") { schema [string] schema { f1: int } }`,
			"has both anonymous schema and field schema",
		},
		{
			"DuplicateSessionField",
			`session("s1") { schema { f1: string f1: int } }`,
			"already defined in session \"s1\"",
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

parameters {
    topic: string
    max_results: int = 5
}

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
        ids: [string]
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
session("gather") { schema { ids: [int] } }
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

func TestCompiler_ComplexValues(t *testing.T) {
	input := `
set config = {
  meta: {
    id: 1,
    tag: "test"
  },
  expr: $(context.val)
}
`
	out, err := compileSource(t, input)
	assert.NoError(t, err)

	vars := out.Vars.Content[1] // Value of 'config'
	var m map[string]interface{}
	vars.Decode(&m)

	assert.Equal(t, "$(context.val)", m["expr"])
	meta := m["meta"].(map[string]interface{})
	assert.Equal(t, 1, meta["id"]) // yaml.v3 decodes small numbers as int
}
