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
parameters {
  limit: int = 10
}
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
	assert.Equal(t, "limit", plan.Statements[1].Parameters.Entries[0].Name)
	assert.Equal(t, "gather", plan.Statements[2].Session.Name)
}

func TestParser_AnonymousSchema(t *testing.T) {
	p, _ := NewParser()
	input := `session("it") { schema [string] }`
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	
	schema := plan.Statements[0].Session.Statements[0].Schema
	assert.NotNil(t, schema.Type)
	assert.NotNil(t, schema.Type.Base.Array)
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
	input := `parameters { status: "up"|"down"|pending }`
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)

	enum := plan.Statements[0].Parameters.Entries[0].Type.Base.Enum
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
