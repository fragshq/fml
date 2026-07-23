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

func TestParser_SystemMultiline(t *testing.T) {
	p, err := NewParser()
	assert.NoError(t, err)

	input := `
system(` + "`" + `
  Line 1
  Line 2
` + "`" + `)
`
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	assert.Contains(t, plan.Statements[0].System.Value, "Line 1")
	assert.Contains(t, plan.Statements[0].System.Value, "Line 2")
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

func TestParser_CallKbs(t *testing.T) {
	p, _ := NewParser()
	input := `call("test") { kbs( my_kb_id ) }`
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)

	kbs := *plan.Statements[0].Call.Fields[0].Kbs
	assert.Contains(t, kbs, "my_kb_id")
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

func TestParser_CallNoBody(t *testing.T) {
	p, _ := NewParser()
	input := `session("s") { call("tool") }`
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)

	stmts := plan.Statements[0].Session.Statements
	assert.Len(t, stmts, 1)
	assert.NotNil(t, stmts[0].Call)
	assert.Equal(t, "tool", stmts[0].Call.Name)
}

func TestParser_UseWithAllowlist(t *testing.T) {
	p, _ := NewParser()
	input := `session("s") { 
  use mcp tool1 { 
    allowlist = ["m1", "m2"] 
  }
}`
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)

	stmts := plan.Statements[0].Session.Statements
	assert.Len(t, stmts, 1)
	assert.NotNil(t, stmts[0].Use)
	assert.Equal(t, "tool1", *stmts[0].Use.Name)
	assert.Len(t, stmts[0].Use.Fields, 1)
	assert.Equal(t, []string{"m1", "m2"}, stmts[0].Use.Fields[0].Allowlist)
}

func TestParser_SetArray(t *testing.T) {
	p, _ := NewParser()
	input := `set tags = ["a", "b", "c"]`
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)

	val := plan.Statements[0].Set.Value
	assert.NotNil(t, val.Array)
	assert.Len(t, val.Array.Values, 3)
}

func TestParser_DashInIdent(t *testing.T) {
	p, _ := NewParser()

	// Test dash in 'set' name
	input1 := `set my-data = "value"`
	plan1, err := p.ParseString("test.frags", input1)
	assert.NoError(t, err)
	assert.Equal(t, "my-data", plan1.Statements[0].Set.Name)

	// Test dash in object key
	input2 := `set config = { my-key: 123 }`
	plan2, err := p.ParseString("test.frags", input2)
	assert.NoError(t, err)
	assert.Equal(t, "my-key", plan2.Statements[0].Set.Value.Object.Entries[0].Key)
}

func TestParser_QuotedKeys(t *testing.T) {
	p, _ := NewParser()

	// Test quoted key in object
	input1 := `set config = { "quoted-key": 456 }`
	plan1, err := p.ParseString("test.frags", input1)
	assert.NoError(t, err)
	assert.Equal(t, "quoted-key", plan1.Statements[0].Set.Value.Object.Entries[0].Key)

	// Test quoted key in schema field
	input2 := `session("s") { schema { "my field": string } }`
	plan2, err := p.ParseString("test.frags", input2)
	assert.NoError(t, err)
	assert.Equal(t, "my field", plan2.Statements[0].Session.Statements[0].Schema.Type.Base.Object.Fields[0].Name)

	// Test quoted key in call argument
	input3 := `call("tool") { "arg-name" = 1 }`
	plan3, err := p.ParseString("test.frags", input3)
	assert.NoError(t, err)
	assert.Equal(t, "arg-name", *plan3.Statements[0].Call.Fields[0].Ident)
}

func TestParser_ErrorPosition_ParserError(t *testing.T) {
	p, _ := NewParser()
	input := `
system("Sys")
invalid_statement
`
	uri := "file:///home/user/test.fml"
	_, err := p.ParseString(uri, input)
	if err != nil {
		pos, msg, ok := ErrorPosition(err)
		if !ok {
			t.Fatal("Expected ErrorPosition to be ok")
		}
		if pos.Line != 3 || pos.Column != 1 {
			t.Errorf("Expected position 3:1, got %d:%d", pos.Line, pos.Column)
		}
		if msg != "unexpected token \"invalid_statement\"" {
			t.Errorf("Expected message 'unexpected token \"invalid_statement\"', got %q", msg)
		}
	} else {
		t.Error("Expected error but got nil")
	}
}

func TestParser_ErrorPosition_LexerError(t *testing.T) {
	p, _ := NewParser()
	input := `
system("Sys")
"unclosed string
`
	uri := "file:///home/user/test.fml"
	_, err := p.ParseString(uri, input)
	if err != nil {
		pos, msg, ok := ErrorPosition(err)
		if !ok {
			t.Fatal("Expected ErrorPosition to be ok")
		}
		if pos.Line != 3 || pos.Column != 1 {
			t.Errorf("Expected position 3:1, got %d:%d", pos.Line, pos.Column)
		}
		if msg != "unterminated string literal" {
			t.Errorf("Expected message 'unterminated string literal', got %q", msg)
		}
	} else {
		t.Error("Expected error but got nil")
	}
}

func TestParser_MultipleComments(t *testing.T) {
	p, err := NewParser()
	assert.NoError(t, err)

	t.Run("top level", func(t *testing.T) {
		input := `
# foo
# bar
system("Sys")
`
		plan, err := p.ParseString("test.frags", input)
		assert.NoError(t, err)
		assert.NotNil(t, plan)
	})

	t.Run("session level", func(t *testing.T) {
		input := `
session("gather") {
  # foo
  # bar
  schema {
    out: string
  }
}
`
		plan, err := p.ParseString("test.frags", input)
		assert.NoError(t, err)
		assert.NotNil(t, plan)
	})

	t.Run("schema block", func(t *testing.T) {
		input := `
session("gather") {
  schema {
    # foo
    # bar
    out: string
  }
}
`
		plan, err := p.ParseString("test.frags", input)
		assert.NoError(t, err)
		assert.NotNil(t, plan)
	})

	t.Run("components block", func(t *testing.T) {
		input := `
components {
  schema("Address") {
    # foo
    # bar
    street: string
  }
}
`
		plan, err := p.ParseString("test.frags", input)
		assert.NoError(t, err)
		assert.NotNil(t, plan)
	})
}

func TestParser_BooleanTypes(t *testing.T) {
	p, _ := NewParser()
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
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	assert.NotNil(t, plan)

	// Check parameters
	assert.Equal(t, "paramBool", plan.Statements[0].Parameter.Name)
	assert.Equal(t, "bool", *plan.Statements[0].Parameter.Attributes[0].Type.Base.Scalar)
	assert.Equal(t, "paramBoolean", plan.Statements[1].Parameter.Name)
	assert.Equal(t, "boolean", *plan.Statements[1].Parameter.Attributes[0].Type.Base.Scalar)

	// Check schema fields
	fields := plan.Statements[2].Session.Statements[0].Schema.Type.Base.Object.Fields
	assert.Len(t, fields, 2)
	assert.Equal(t, "f1", fields[0].Name)
	assert.Equal(t, "bool", *fields[0].Type.Base.Scalar)
	assert.Equal(t, "f2", fields[1].Name)
	assert.Equal(t, "boolean", *fields[1].Type.Base.Scalar)
}
