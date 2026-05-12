package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParser_FullPlan(t *testing.T) {
	p, err := NewParser()
	assert.NoError(t, err)

	input := `
system("Sys")
parameter("limit", type=int, default=10)
session("gather") {
  schema {
    out: string
  }
}
`
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	assert.NotNil(t, plan)

	assert.Equal(t, "Sys", plan.Statements[0].System.Value)
	assert.Equal(t, "limit", plan.Statements[1].Parameter.Name)
	assert.Equal(t, "gather", plan.Statements[2].Session.Name)
}

func TestParser_AnonymousSchema(t *testing.T) {
	p, _ := NewParser()
	input := `session("it") { schema string[] }`
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)

	schema := plan.Statements[0].Session.Statements[0].Schema
	assert.NotNil(t, schema.Type)
	assert.True(t, schema.Type.Suffix)
}

func TestParser_CallCode(t *testing.T) {
	p, _ := NewParser()
	input := `call("test") { code( args.x.map(y => y * 2) ) }`
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)

	code := *plan.Statements[0].Call.Fields[0].Code
	assert.Contains(t, code, "args.x.map")
}

func TestParser_EnumTypes(t *testing.T) {
	p, _ := NewParser()
	input := `parameter("status", type="up"|"down"|pending)`
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)

	var enum []string
	for _, attr := range plan.Statements[0].Parameter.Attributes {
		if attr.Type != nil {
			enum = attr.Type.Base.Enum
		}
	}
	assert.Equal(t, []string{"up", "down", "pending"}, enum)
}

func TestParser_ComplexValue(t *testing.T) {
	p, _ := NewParser()
	input := `set config = { limit: 10, debug: true, path: $(context.base.path) }`
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)

	val := plan.Statements[0].Set.Value
	assert.NotNil(t, val.Object)
	assert.Len(t, val.Object.Entries, 3)
	assert.Equal(t, "limit", val.Object.Entries[0].Key)
	assert.Equal(t, "context.base.path", *val.Object.Entries[2].Value.Expr)
}

func TestParser_SessionTarget(t *testing.T) {
	p, _ := NewParser()
	input := `session("s1", target="output") { }`
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)

	attr := plan.Statements[0].Session.Attributes[0]
	assert.Equal(t, "target", attr.Type)
	assert.Equal(t, "output", attr.Value)
}

func TestParser_RootRequire(t *testing.T) {
	p, _ := NewParser()
	input := `require mcp tool1
require search`
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)

	assert.Len(t, plan.Statements, 2)
	assert.NotNil(t, plan.Statements[0].Require)
	assert.Equal(t, "tool1", *plan.Statements[0].Require.Name)
	assert.True(t, plan.Statements[1].Require.Search)
}
