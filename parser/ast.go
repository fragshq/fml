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
	Parameters  *ParametersBlock  `| @@`
	Components  *ComponentsBlock  `| @@`
	Session     *SessionBlock     `| @@`
	Call        *CallBlock        `| @@`
	Transformer *TransformerBlock `| @@`
	Set         *SetStmt          `| @@`
}

type SystemBlock struct {
	Value         string  `"system" "(" @String ")"`
	InlineComment *string `@InlineComment?`
}

type ParametersBlock struct {
	Entries       []*ParamEntry `"parameters" "{" (@@ (","? @@)*)? "}"`
	InlineComment *string       `@InlineComment?`
}

// ParamEntry defines a single input parameter with optional documentation and default values.
type ParamEntry struct {
	LeadingComments []string  `@Comment*`
	Name            string    `@Ident ":"`
	Type            *TypeExpr `@@`
	Default         *Value    `("=" @@)?`
	InlineComment   *string   `@InlineComment?`
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
	InlineComment *string `@InlineComment?`
}

// CallBlock represents a function or tool invocation, mapping inputs to an optional variable.
type CallBlock struct {
	Name          string       `"call" "(" @String ")"`
	Target        *CallTarget  `("->" @@)?`
	InlineComment *string      `@InlineComment?`
	Fields        []*CallField `"{" @@* "}"`
}

type CallTarget struct {
	Namespace *string `(@Ident ":")?`
	Name      string  `@Ident`
}

type CallField struct {
	Code          *string `("code" "(" @CodeValue ")"`
	Ident         *string `| @Ident "="`
	Value         *Value  `@@)`
	InlineComment *string `@InlineComment?`
}

type SetStmt struct {
	Name          string  `"set" @Ident "="`
	Value         *Value  `@@`
	InlineComment *string `@InlineComment?`
}

type Value struct {
	String *string   `@String`
	Number *float64  `| @Number`
	Bool   *bool     `| @Bool`
	Expr   *string   `| "$(" @CodeValue ")"`
	Object *MapValue `| "{" @@ "}"`
}

type MapValue struct {
	Entries []*MapEntry `(@@ (","? @@)*)?`
}

type MapEntry struct {
	Key   string `@Ident ":"`
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
	Name   string         `"schema" "(" @String ")" "{"`
	Fields []*SchemaField `(@@ (","? @@)*)? "}"`
}

type PromptComponent struct {
	Name  string `"prompt" "(" @String ")" "{"`
	Value string `@String "}"`
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
	Comment *string      `@Comment`
	Set     *SetStmt     `| @@`
	Use     *UseStmt     `| @@`
	Call    *CallBlock   `| @@`
	Context *ContextStmt `| @@`
	Schema  *SchemaBlock `| @@`
	Prompt  *PromptLines `| @@`
}

type UseStmt struct {
	Search        bool    `("use" @("search")`
	Type          *string `| "use" @("mcp" | "apicp" | "collection" | "function")`
	Name          *string `@Ident)`
	InlineComment *string `@InlineComment?`
}

type ContextStmt struct {
	Value         *Value  `"context" @@`
	InlineComment *string `@InlineComment?`
}

// PromptLines represents a sequence of indentation-sensitive LLM prompt items.
type PromptLines struct {
	Items []string `@PromptItem+`
}

// SchemaBlock defines the output structure of a session or component.
type SchemaBlock struct {
	Fields        []*SchemaField `("schema" "{" (@@ (","? @@)*)? "}")`
	Type          *TypeExpr      `| ("schema" @@)`
	InlineComment *string        `@InlineComment?`
}

type SchemaField struct {
	LeadingComments []string  `@Comment*`
	Name            string    `@Ident`
	Optional        bool      `@("?")? ":"`
	Type            *TypeExpr `@@`
	InlineComment   *string   `@InlineComment?`
}

// TypeExpr is the unified type representation, supporting scalars, arrays, objects, and refs.
type TypeExpr struct {
	Base     *TypeBase `@@`
	Suffix   bool      `@("[" "]")?`
	Optional bool      `@("?")?`
}

type TypeBase struct {
	Scalar *string     `@("string" | "int" | "float" | "bool" | "any")`
	Array  *TypeExpr   `| "[" @@ "]"`
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
		participle.Unquote("String"),
		participle.UseLookahead(10),
	)
}
