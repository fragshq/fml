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
	Name            string           `"parameter" "(" @String`
	Attributes      []*ParameterAttr `("," @@)* ")" `
	InlineComment   *string          `@InlineComment?`
}

type ParameterAttr struct {
	Type    *TypeExpr `( "type" "=" @@`
	Default *Value    `| "default" "=" @@`
	Title   *string   `| "title" "=" @String`
	Enum    *TypeExpr `| "enum" "=" @@ )`
}

type TransformerBlock struct {
	Name          string              `"transformer" "(" @String ")" "{"`
	InlineComment *string             `@InlineComment?`
	Fields        []*TransformerField `@@* "}"`
}

type TransformerField struct {
	TriggerType   *string `(@("onFunctionOutput" | "onFunctionInput" | "onResource")`
	TriggerValue  *string `"=" @String)`
	JMESPath      *string `| ("jmesPath" "=" @String)`
	Parser        *string `| ("parser" "=" @("json" | "csv" | String))`
	Code          *string `| ("code" "(" @CodeValue ")")`
	InlineComment *string `@InlineComment?`
}

// CallBlock represents a function or tool invocation, mapping inputs to an optional variable.
type CallBlock struct {
	Name          string       `"call" "(" @String ")"`
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
	Ident         *string `| (@Ident | @String) "="`
	Value         *Value  `@@)`
	InlineComment *string `@InlineComment?`
}

type SetStmt struct {
	Name          string  `"set" @Ident "="`
	Value         *Value  `@@`
	InlineComment *string `@InlineComment?`
}

type Value struct {
	String *string     `@String`
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
	Key   string `(@Ident | @String) ":"`
	Value *Value `@@`
}

type ComponentsBlock struct {
	Items []*ComponentItem `"components" "{" @@* "}"`
}

type ComponentItem struct {
	Schema *SchemaComponent `@@`
	Prompt *PromptComponent `| @@`
}

type SchemaComponent struct {
	LeadingComments []string       `@Comment*`
	Name            string         `"schema" "(" @String ")" "{"`
	Fields          []*SchemaField `(@@ (","? @@)*)? "}"`
	InlineComment   *string        `@InlineComment?`
}

type PromptComponent struct {
	LeadingComments []string `@Comment*`
	Name            string   `"prompt" "(" @String ")" "{"`
	Value           string   `@String "}"`
	InlineComment   *string  `@InlineComment?`
}

// SessionBlock represents a logical pipeline step with its own context, tools, and output schema.
type SessionBlock struct {
	Name          string         `"session" "(" @String`
	Attributes    []*SessionAttr `@@* ")" "{"`
	InlineComment *string        `@InlineComment?`
	Statements    []*SessionStmt `@@* "}"`
}

// SessionAttr handles session configuration such as dependencies (after/expect), iteration, or schema target renaming.
type SessionAttr struct {
	Type  string `"," @("after" | "expect" | "iterate" | "target") "="`
	Value string `(@String | @AttrValue)`
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
	Identifier    string      `"resource" @String`
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
	Allowlist []string `"allowlist" "=" "[" @String ("," @String)* "]"`
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
	Name            string    `(@Ident | @String)`
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
	Enum   []string    `| (@(Ident|String) ("|" @(Ident|String))* )`
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
