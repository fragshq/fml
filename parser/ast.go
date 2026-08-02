package parser

import (
	"github.com/alecthomas/participle/v2"
)

// Plan is the root node of the Frags DSL AST.
type Plan struct {
	Statements []*Statement `@@*`
}

// Statement represents a top-level block or standalone statement in a plan.
type Statement struct {
	Comment     *string           `@Comment`
	System      *SystemBlock      `| @@`
	Parameter   *ParameterBlock   `| @@`
	Components  *ComponentsBlock  `| @@`
	Session     *SessionBlock     `| @@`
	Call        *CallBlock        `| @@`
	Transformer *TransformerBlock `| @@`
	Set         *SetStmt          `| @@`
	Require     *RequireStmt      `| @@`
}

type RequireStmt struct {
	Search        bool    `("require" @("search")`
	Type          *string `| "require" @("mcp" | "apicp" | "collection" | "function")`
	Name          *string `@Ident)`
	InlineComment *string `@InlineComment?`
}

type SystemBlock struct {
	Value         string  `"system" "(" (@String | @RawString) ")"`
	InlineComment *string `@InlineComment?`
}

type ParameterBlock struct {
	LeadingComments []string         `@Comment*`
	Name            string           `"parameter" "(" (@String | @RawString)`
	Attributes      []*ParameterAttr `("," @@)* ")" `
	InlineComment   *string          `@InlineComment?`
}

type ParameterAttr struct {
	Type    *TypeExpr `( "type" "=" @@`
	Default *Value    `| "default" "=" @@`
	Title   *string   `| "title" "=" (@String | @RawString)`
	Enum    *TypeExpr `| "enum" "=" @@ )`
}

type TransformerBlock struct {
	Name          string              `"transformer" "(" (@String | @RawString) ")" "{"`
	InlineComment *string             `@InlineComment?`
	Fields        []*TransformerField `@@* "}"`
}

type TransformerField struct {
	TriggerType   *string `(@("onFunctionOutput" | "onFunctionInput" | "onResource")`
	TriggerValue  *string `"=" (@String | @RawString))`
	JMESPath      *string `| ("jmesPath" "=" (@String | @RawString))`
	Parser        *string `| ("parser" "=" @("json" | "csv" | String | RawString))`
	Code          *string `| ("code" "(" @CodeValue ")")`
	InlineComment *string `@InlineComment?`
}

// CallBlock represents a function or tool invocation, mapping inputs to an optional variable.
type CallBlock struct {
	Name          string       `"call" "(" (@String | @RawString) ")"`
	Target        *CallTarget  `("->" @@)?`
	InlineComment *string      `@InlineComment?`
	Fields        []*CallField `("{" @@* "}")?`
}

type CallTarget struct {
	Namespace *string `(@Ident ":")?`
	Name      string  `@Ident`
}

type CallField struct {
	Code          *string `("code" "(" @CodeValue ")"`
	Kbs           *string `| "kbs" "(" @CodeValue ")"`
	Ident         *string `| (@Ident | @String | @RawString) "="`
	Value         *Value  `@@)`
	InlineComment *string `@InlineComment?`
}

type SetStmt struct {
	Name          string  `"set" @Ident "="`
	Value         *Value  `@@`
	InlineComment *string `@InlineComment?`
}

type Value struct {
	String *string     `(@String | @RawString)`
	Number *float64    `| @Number`
	Bool   *bool       `| @Bool`
	Expr   *string     `| "$(" @CodeValue ")"`
	Object *MapValue   `| "{" @@ "}"`
	Array  *ArrayValue `| "[" @@ "]"`
}

type ArrayValue struct {
	Values []*Value `(@@ (","? @@)*)?`
}

type MapValue struct {
	Entries []*MapEntry `(@@ (","? @@)*)?`
}

type MapEntry struct {
	Key   string `(@Ident | @String | @RawString) ":"`
	Value *Value `@@`
}

type ComponentsBlock struct {
	Items []*ComponentItem `"components" "{" @@* "}"`
}

type ComponentItem struct {
	Schema *SchemaComponent `@@`
	Prompt *PromptComponent `| @@`
	Script *ScriptComponent `| @@`
}

type SchemaComponent struct {
	LeadingComments []string       `@Comment*`
	Name            string         `"schema" "(" (@String | @RawString) ")" "{"`
	Fields          []*SchemaField `(@@ (","? @@)*)? "}"`
	InlineComment   *string        `@InlineComment?`
}

type PromptComponent struct {
	LeadingComments []string `@Comment*`
	Name            string   `"prompt" "(" (@String | @RawString) ")" "{"`
	Value           string   `(@String | @RawString) "}"`
	InlineComment   *string  `@InlineComment?`
}

type ScriptComponent struct {
	LeadingComments []string      `@Comment*`
	Name            string        `"script" "(" (@String | @RawString)`
	Type            string        `"," "type" "=" (@String | @RawString)`
	Description     *string       `("," "description" "=" (@String | @RawString))?`
	Parameters      *ScriptParams `("," "parameters" "=" @@)? ")"`
	Body            string        `"(" @CodeValue ")"`
	InlineComment   *string       `@InlineComment?`
}

type ScriptParams struct {
	Entries []*ScriptParamEntry `"{" (@@ (","? @@)*)? "}"`
}

type ScriptParamEntry struct {
	Name string    `(@Ident | @String | @RawString) ":"`
	Type *TypeExpr `@@`
}

// SessionBlock represents a logical pipeline step with its own context, tools, and output schema.
type SessionBlock struct {
	Name          string         `"session" "(" (@String | @RawString)`
	Attributes    []*SessionAttr `@@* ")" "{"`
	InlineComment *string        `@InlineComment?`
	Statements    []*SessionStmt `@@* "}"`
}

// SessionAttr handles session configuration such as dependencies (after/expect), iteration, or schema target renaming.
type SessionAttr struct {
	Type  string `"," @("after" | "expect" | "iterate" | "target") "="`
	Value string `(@String | @RawString | @AttrValue)`
}

type SessionStmt struct {
	Comment  *string       `@Comment`
	Set      *SetStmt      `| @@`
	Use      *UseStmt      `| @@`
	Call     *CallBlock    `| @@`
	Context  *ContextStmt  `| @@`
	Resource *ResourceStmt `| @@`
	Schema   *SchemaBlock  `| @@`
	Prompt   *PromptLines  `| @@`
}

type ResourceStmt struct {
	Identifier    string      `"resource" (@String | @RawString)`
	Target        *CallTarget `("->" @@)?`
	InlineComment *string     `@InlineComment?`
}

type UseStmt struct {
	Search        bool        `("use" @("search")`
	Type          *string     `| "use" @("mcp" | "apicp" | "collection" | "function")`
	Name          *string     `@Ident)`
	Fields        []*UseField `("{" @@* "}")?`
	InlineComment *string     `@InlineComment?`
}

type UseField struct {
	Allowlist []string `"allowlist" "=" "[" (@String | @RawString) ("," (@String | @RawString))* "]"`
}

type ContextStmt struct {
	Value         *Value  `"context" @@`
	InlineComment *string `@InlineComment?`
}

// PromptLines represents a sequence of indentation-sensitive LLM prompt items.
type PromptLines struct {
	Items []*PromptLine `@@+`
}

type PromptLine struct {
	PrePrompt *string `@PrePromptItem`
	Prompt    *string `| @PromptItem`
}

// SchemaBlock defines the output structure of a session or component.
type SchemaBlock struct {
	LeadingComments []string  `@Comment*`
	Optional        bool      `"schema" @("?")?`
	Type            *TypeExpr `@@`
	InlineComment   *string   `@InlineComment?`
}

type SchemaField struct {
	LeadingComments []string  `@Comment*`
	Name            string    `(@Ident | @String | @RawString)`
	Optional        bool      `@("?")? ":"`
	Type            *TypeExpr `@@`
	InlineComment   *string   `@InlineComment?`
}

// TypeExpr is the unified type representation, supporting scalars, arrays, objects, and refs.
type TypeExpr struct {
	Base   *TypeBase `@@`
	Suffix bool      `@("[" "]")?`
}

type TypeBase struct {
	Scalar *string     `@("string" | "int" | "float" | "bool" | "boolean" | "any")`
	Object *ObjectBody `| @@`
	Ref    *string     `| "$" @Ident`
	Enum   []string    `| (@(Ident|String|RawString) ("|" @(Ident|String|RawString))* )`
}

type ObjectBody struct {
	Fields []*SchemaField `"{" (@@ (","? @@)*)? "}"`
}

// NewParser initializes the Frags DSL parser with the custom stateful lexer.
func NewParser() (*participle.Parser[Plan], error) {
	return participle.Build[Plan](
		participle.Lexer(&FRAGSLexerDefinition{}),
		participle.Unquote("String", "RawString"),
		participle.UseLookahead(10),
	)
}
