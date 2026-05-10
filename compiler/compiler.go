package compiler

import (
	"fmt"
	"strings"

	"github.com/theirish/frags-compiler/parser"
	"gopkg.in/yaml.v3"
)

// Compiler transforms a Frags DSL AST into a validated PlanYAML structure with metadata preservation.
type Compiler struct {
	plan            *parser.Plan
	output          *PlanYAML
	sessionSchemas  map[string]*JSONSchema
	sessionOrder    []string
	sessionOptional map[string]bool
	pendingComments []string // Orphaned comments to be attached as HeadComments
}

func New(plan *parser.Plan) *Compiler {
	return &Compiler{
		plan: plan,
		output: &PlanYAML{
			Sessions: &yaml.Node{Kind: yaml.MappingNode},
		},
		sessionSchemas:  make(map[string]*JSONSchema),
		sessionOptional: make(map[string]bool),
	}
}

// Compile orchestrates the transformation, accurately mapping DSL structures to YAML nodes.
func (c *Compiler) Compile() (*PlanYAML, error) {
	rootSchema := &JSONSchema{
		Type:       "object",
		Properties: make(map[string]*JSONSchema),
	}

	for _, stmt := range c.plan.Statements {
		if stmt.Components != nil {
			if err := c.processComponents(stmt.Components); err != nil {
				return nil, err
			}
		}
	}

	for _, stmt := range c.plan.Statements {
		switch {
		case stmt.Comment != nil:
			c.collectComment(*stmt.Comment)
		case stmt.System != nil:
			c.output.SystemPrompt = c.nodeValue(stmt.System.Value, stmt.System.InlineComment)
		case stmt.Parameters != nil:
			if err := c.processParameters(stmt.Parameters); err != nil {
				return nil, err
			}
		case stmt.Transformer != nil:
			if err := c.processTransformer(stmt.Transformer); err != nil {
				return nil, err
			}
		case stmt.Set != nil:
			if c.output.Vars == nil {
				c.output.Vars = &yaml.Node{Kind: yaml.MappingNode}
			}
			c.addNodeMapEntry(c.output.Vars, stmt.Set.Name, c.compileValue(stmt.Set.Value), stmt.Set.InlineComment)
		case stmt.Call != nil:
			callNode, err := c.compileCallNode(stmt.Call)
			if err != nil {
				return nil, err
			}
			if c.output.PreCalls == nil {
				c.output.PreCalls = &yaml.Node{Kind: yaml.SequenceNode}
			}
			c.output.PreCalls.Content = append(c.output.PreCalls.Content, callNode)
		case stmt.Session != nil:
			if err := c.processSession(stmt.Session); err != nil {
				return nil, err
			}
		}
	}

	// Grouped session schema finalization
	for _, name := range c.sessionOrder {
		sessNode := c.getSessionNode(name)
		schema, ok := c.sessionSchemas[name]
		if !ok {
			continue
		}

		// Iterate logic: entire session result is a collection.
		iterateOn := ""
		for i := 0; i < len(sessNode.Content); i += 2 {
			if sessNode.Content[i].Value == "iterateOn" {
				iterateOn = sessNode.Content[i+1].Value
			}
		}

		if iterateOn != "" && !c.isTypeArray(schema) {
			schema = &JSONSchema{
				Type:  "array",
				Items: schema,
			}
		}

		schema.XSession = name
		rootSchema.Properties[name] = schema
		
		if !c.sessionOptional[name] {
			rootSchema.Required = append(rootSchema.Required, name)
		}
	}

	c.finalizeRequiredTools()

	if len(rootSchema.Properties) > 0 {
		var schemaNode yaml.Node
		schemaNode.Encode(rootSchema)
		c.output.Schema = &schemaNode
	}

	return c.output, nil
}

func (c *Compiler) collectComment(s string) {
	c.pendingComments = append(c.pendingComments, strings.TrimSpace(strings.TrimPrefix(s, "#")))
}

// nodeValue encodes a value and attaches any pending or inline comments.
func (c *Compiler) nodeValue(v interface{}, inline *string) *yaml.Node {
	var node yaml.Node
	node.Encode(v)
	if len(c.pendingComments) > 0 {
		node.HeadComment = strings.Join(c.pendingComments, "\n")
		c.pendingComments = nil
	}
	if inline != nil {
		node.LineComment = strings.TrimSpace(strings.TrimPrefix(*inline, "#"))
	}
	return &node
}

// addNodeMapEntry adds a key-value pair to a MappingNode, preserving comment context.
func (c *Compiler) addNodeMapEntry(mapNode *yaml.Node, key string, val interface{}, inline *string) {
	var keyNode yaml.Node
	keyNode.Encode(key)
	if len(c.pendingComments) > 0 {
		keyNode.HeadComment = strings.Join(c.pendingComments, "\n")
		c.pendingComments = nil
	}
	if inline != nil {
		keyNode.LineComment = strings.TrimSpace(strings.TrimPrefix(*inline, "#"))
	}
	
	var valNode yaml.Node
	valNode.Encode(val)
	mapNode.Content = append(mapNode.Content, &keyNode, &valNode)
}

func (c *Compiler) processParameters(p *parser.ParametersBlock) error {
	if c.output.Parameters == nil {
		c.output.Parameters = &yaml.Node{Kind: yaml.SequenceNode}
	}
	// Attach block-level comments to the 'parameters' key if they exist.
	// But parameters is a slice of ParameterYAML, so we handle comments per entry.
	for _, entry := range p.Entries {
		param := &ParameterYAML{
			Name:   entry.Name,
			Schema: c.compileType(entry.Type),
		}
		if entry.Default != nil {
			param.Default = c.nodeValue(c.compileValue(entry.Default), nil)
		}
		
		desc := c.resolveDescription(entry.LeadingComments, entry.InlineComment)
		if desc != "" {
			param.Schema.Description = desc
		}

		node := c.nodeValue(param, nil)
		c.output.Parameters.Content = append(c.output.Parameters.Content, node)
	}
	return nil
}

func (c *Compiler) processTransformer(t *parser.TransformerBlock) error {
	if c.output.Transformers == nil {
		c.output.Transformers = &yaml.Node{Kind: yaml.SequenceNode}
	}
	trans := &TransformerYAML{Name: t.Name}
	for _, field := range t.Fields {
		if field.JMESPath != nil {
			trans.JMESPath = *field.JMESPath
		} else if field.TriggerType != nil && field.TriggerValue != nil {
			val := strings.Trim(*field.TriggerValue, "\"")
			switch *field.TriggerType {
			case "on_function": trans.OnFunctionOutput = val
			case "on_input":    trans.OnFunctionInput = val
			case "on_resource": trans.OnResource = val
			}
		}
	}
	node := c.nodeValue(trans, t.InlineComment)
	c.output.Transformers.Content = append(c.output.Transformers.Content, node)
	return nil
}

func (c *Compiler) processComponents(comp *parser.ComponentsBlock) error {
	if c.output.Components == nil {
		c.output.Components = &ComponentsYAML{
			Schemas: make(map[string]*JSONSchema),
			Prompts: make(map[string]*yaml.Node),
		}
	}
	for _, item := range comp.Items {
		if item.Schema != nil {
			schema := &JSONSchema{
				Type:       "object",
				Properties: make(map[string]*JSONSchema),
			}
			for _, field := range item.Schema.Fields {
				fSchema := c.compileType(field.Type)
				desc := c.resolveDescription(field.LeadingComments, field.InlineComment)
				if desc != "" { fSchema.Description = desc }
				schema.Properties[field.Name] = fSchema
				if !field.Optional && !field.Type.Optional {
					schema.Required = append(schema.Required, field.Name)
				}
			}
			c.output.Components.Schemas[item.Schema.Name] = schema
		} else if item.Prompt != nil {
			c.output.Components.Prompts[item.Prompt.Name] = c.nodeValue(item.Prompt.Value, nil)
		}
	}
	return nil
}

func (c *Compiler) getSessionNode(name string) *yaml.Node {
	for i := 0; i < len(c.output.Sessions.Content); i += 2 {
		if c.output.Sessions.Content[i].Value == name {
			return c.output.Sessions.Content[i+1]
		}
	}
	return nil
}

func (c *Compiler) processSession(s *parser.SessionBlock) error {
	var sessNode *yaml.Node
	for i := 0; i < len(c.output.Sessions.Content); i += 2 {
		if c.output.Sessions.Content[i].Value == s.Name {
			sessNode = c.output.Sessions.Content[i+1]
			break
		}
	}

	if sessNode == nil {
		// New session: create key-value pair in MappingNode
		var keyNode yaml.Node
		keyNode.Encode(s.Name)
		if len(c.pendingComments) > 0 {
			keyNode.HeadComment = strings.Join(c.pendingComments, "\n")
			c.pendingComments = nil
		}
		if s.InlineComment != nil {
			keyNode.LineComment = strings.TrimSpace(strings.TrimPrefix(*s.InlineComment, "#"))
		}
		
		sessNode = &yaml.Node{Kind: yaml.MappingNode}
		c.output.Sessions.Content = append(c.output.Sessions.Content, &keyNode, sessNode)
		c.sessionOrder = append(c.sessionOrder, s.Name)
	}

	if _, exists := c.sessionSchemas[s.Name]; !exists {
		c.sessionSchemas[s.Name] = &JSONSchema{
			Type:       "object",
			Properties: make(map[string]*JSONSchema),
		}
	}
	sessSchema := c.sessionSchemas[s.Name]

	for _, attr := range s.Attributes {
		switch attr.Type {
		case "after":
			c.addDependsOn(sessNode, attr.Value, "")
		case "expect":
			c.addDependsOn(sessNode, "", attr.Value)
		case "iterate":
			c.setNodeMapField(sessNode, "iterateOn", attr.Value)
		}
	}

	var promptItems []string

	for _, stmt := range s.Statements {
		switch {
		case stmt.Comment != nil:
			c.collectComment(*stmt.Comment)
		case stmt.Set != nil:
			varsNode := c.ensureNodeMapField(sessNode, "vars")
			c.addNodeMapEntry(varsNode, stmt.Set.Name, c.compileValue(stmt.Set.Value), stmt.Set.InlineComment)
		case stmt.Use != nil:
			toolsNode := c.ensureNodeSeqField(sessNode, "tools")
			tool := &ToolYAML{}
			if stmt.Use.Search {
				tool.Type = "internet_search"
			} else {
				if stmt.Use.Type != nil { tool.Type = *stmt.Use.Type }
				if stmt.Use.Name != nil { tool.Name = *stmt.Use.Name }
			}
			toolsNode.Content = append(toolsNode.Content, c.nodeValue(tool, stmt.Use.InlineComment))
		case stmt.Call != nil:
			callsNode := c.ensureNodeSeqField(sessNode, "preCalls")
			callNode, err := c.compileCallNode(stmt.Call)
			if err != nil { return err }
			callsNode.Content = append(callsNode.Content, callNode)
		case stmt.Context != nil:
			c.setNodeMapFieldNode(sessNode, "context", c.nodeValue(c.compileValue(stmt.Context.Value), stmt.Context.InlineComment))
		case stmt.Prompt != nil:
			promptItems = append(promptItems, stmt.Prompt.Items...)
		case stmt.Schema != nil:
			if stmt.Schema.Type != nil {
				c.sessionSchemas[s.Name] = c.compileType(stmt.Schema.Type)
				c.sessionOptional[s.Name] = stmt.Schema.Type.Optional
			} else {
				for _, field := range stmt.Schema.Fields {
					fSchema := c.compileType(field.Type)
					desc := c.resolveDescription(field.LeadingComments, field.InlineComment)
					if desc != "" { fSchema.Description = desc }
					
					if sessSchema.Properties == nil {
						return fmt.Errorf("session %q has both anonymous schema and field schema", s.Name)
					}
					if _, exists := sessSchema.Properties[field.Name]; exists {
						return fmt.Errorf("field %q already defined in session %q", field.Name, s.Name)
					}
					sessSchema.Properties[field.Name] = fSchema
					if !field.Optional && !field.Type.Optional {
						sessSchema.Required = append(sessSchema.Required, field.Name)
					}
				}
			}
		}
	}

	if len(promptItems) > 0 {
		cleaned := make([]string, len(promptItems))
		for i, p := range promptItems {
			cleaned[i] = c.cleanPromptItem(p)
		}
		
		if len(cleaned) == 1 {
			c.setNodeMapFieldNode(sessNode, "prompt", c.nodeValue(cleaned[0], nil))
		} else if len(cleaned) == 2 {
			c.setNodeMapFieldNode(sessNode, "prePrompt", c.nodeValue(cleaned[0], nil))
			c.setNodeMapFieldNode(sessNode, "prompt", c.nodeValue(cleaned[1], nil))
		} else {
			c.setNodeMapFieldNode(sessNode, "prePrompt", c.nodeValue(cleaned[:len(cleaned)-1], nil))
			c.setNodeMapFieldNode(sessNode, "prompt", c.nodeValue(cleaned[len(cleaned)-1], nil))
		}
	}

	return nil
}

func (c *Compiler) ensureNodeMapField(parent *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			return parent.Content[i+1]
		}
	}
	var kn, vn yaml.Node
	kn.Encode(key)
	vn.Kind = yaml.MappingNode
	parent.Content = append(parent.Content, &kn, &vn)
	return &vn
}

func (c *Compiler) ensureNodeSeqField(parent *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			return parent.Content[i+1]
		}
	}
	var kn, vn yaml.Node
	kn.Encode(key)
	vn.Kind = yaml.SequenceNode
	parent.Content = append(parent.Content, &kn, &vn)
	return &vn
}

func (c *Compiler) setNodeMapField(parent *yaml.Node, key string, val interface{}) {
	for i := 0; i < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			parent.Content[i+1].Encode(val)
			return
		}
	}
	var kn, vn yaml.Node
	kn.Encode(key)
	vn.Encode(val)
	parent.Content = append(parent.Content, &kn, &vn)
}

func (c *Compiler) setNodeMapFieldNode(parent *yaml.Node, key string, vn *yaml.Node) {
	for i := 0; i < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			parent.Content[i+1] = vn
			return
		}
	}
	var kn yaml.Node
	kn.Encode(key)
	parent.Content = append(parent.Content, &kn, vn)
}

func (c *Compiler) addDependsOn(sessNode *yaml.Node, session string, expression string) {
	depsNode := c.ensureNodeSeqField(sessNode, "dependsOn")
	if session != "" {
		dep := &DependsOnYAML{Session: session}
		depsNode.Content = append(depsNode.Content, c.nodeValue(dep, nil))
	} else if expression != "" {
		if len(depsNode.Content) > 0 {
			last := depsNode.Content[len(depsNode.Content)-1]
			// Check if last already has expression
			found := false
			for i := 0; i < len(last.Content); i += 2 {
				if last.Content[i].Value == "expression" {
					found = true
					break
				}
			}
			if !found {
				c.addNodeMapEntry(last, "expression", expression, nil)
				return
			}
		}
		dep := &DependsOnYAML{Expression: expression}
		depsNode.Content = append(depsNode.Content, c.nodeValue(dep, nil))
	}
}

func (c *Compiler) compileCallNode(call *parser.CallBlock) (*yaml.Node, error) {
	y := &CallYAML{
		Name: call.Name,
		Args: make(map[string]interface{}),
	}
	if call.Target != nil {
		y.In = "vars"
		y.Var = *call.Target
	} else {
		y.In = "ai"
	}
	for _, field := range call.Fields {
		if field.Ident != nil {
			y.Args[*field.Ident] = c.compileValue(field.Value)
		} else if field.Code != nil {
			y.Code = strings.TrimSpace(*field.Code)
		}
	}
	return c.nodeValue(y, call.InlineComment), nil
}

func (c *Compiler) cleanPromptItem(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) == 0 { return "" }
	first := lines[0]
	i := strings.Index(first, "- ")
	if i != -1 { first = first[i+2:] } else if i = strings.Index(first, "-"); i != -1 { first = first[i+1:] }
	var result []string
	result = append(result, strings.TrimRight(first, " \t"))
	if len(lines) > 1 {
		fullFirst := lines[0]
		indent := 0
		for indent < len(fullFirst) && (fullFirst[indent] == ' ' || fullFirst[indent] == '\t') { indent++ }
		stripLen := indent + 1
		for _, line := range lines[1:] {
			if len(line) > stripLen { result = append(result, strings.TrimRight(line[stripLen:], " \t")) } else { result = append(result, "") }
		}
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}

func (c *Compiler) compileType(t *parser.TypeExpr) *JSONSchema {
	if t == nil { return &JSONSchema{} }
	schema := c.compileTypeBase(t.Base)
	if t.Suffix { schema = &JSONSchema{Type: "array", Items: schema} }
	return schema
}

func (c *Compiler) compileTypeBase(t *parser.TypeBase) *JSONSchema {
	if t == nil { return &JSONSchema{} }
	switch {
	case t.Scalar != nil:
		st := *t.Scalar
		if st == "int" { st = "integer" } else if st == "float" { st = "number" }
		if st == "any" { return &JSONSchema{} }
		return &JSONSchema{Type: st}
	case t.Array != nil:
		inner := c.compileType(t.Array)
		return &JSONSchema{Type:  "array", Items: inner}
	case t.Object != nil:
		schema := &JSONSchema{Type: "object", Properties: make(map[string]*JSONSchema)}
		for _, field := range t.Object.Fields {
			fSchema := c.compileType(field.Type)
			schema.Properties[field.Name] = fSchema
			if !field.Optional && !field.Type.Optional { schema.Required = append(schema.Required, field.Name) }
		}
		return schema
	case t.Ref != nil:
		return &JSONSchema{Ref: fmt.Sprintf("#/components/schemas/%s", *t.Ref)}
	case len(t.Enum) > 0:
		enum := make([]interface{}, len(t.Enum))
		for i, e := range t.Enum { enum[i] = e }
		return &JSONSchema{ Type: "string", Enum: enum }
	}
	return &JSONSchema{}
}

func (c *Compiler) compileValue(v *parser.Value) interface{} {
	if v == nil { return nil }
	switch {
	case v.String != nil: return *v.String
	case v.Number != nil: return *v.Number
	case v.Bool != nil:   return *v.Bool
	case v.Expr != nil:   return fmt.Sprintf("$(%s)", *v.Expr)
	}
	return nil
}

func (c *Compiler) resolveDescription(leading []string, inline *string) string {
	if inline != nil { return strings.TrimSpace(strings.TrimPrefix(*inline, "#")) }
	if len(leading) > 0 {
		var lines []string
		for _, l := range leading { lines = append(lines, strings.TrimSpace(strings.TrimPrefix(l, "#"))) }
		return strings.Join(lines, " ")
	}
	return ""
}

func (c *Compiler) isTypeArray(s *JSONSchema) bool {
	if s.Type == "array" { return true }
	if s.Ref != "" {
		name := strings.TrimPrefix(s.Ref, "#/components/schemas/")
		if c.output.Components != nil && c.output.Components.Schemas != nil {
			if comp, ok := c.output.Components.Schemas[name]; ok { return comp.Type == "array" }
		}
	}
	return false
}

func (c *Compiler) finalizeRequiredTools() {
	seen := make(map[string]bool)
	for i := 1; i < len(c.output.Sessions.Content); i += 2 {
		sessNode := c.output.Sessions.Content[i]
		var toolsNode *yaml.Node
		for j := 0; j < len(sessNode.Content); j += 2 {
			if sessNode.Content[j].Value == "tools" {
				toolsNode = sessNode.Content[j+1]
				break
			}
		}
		if toolsNode == nil { continue }
		for _, tn := range toolsNode.Content {
			var typeVal, nameVal string
			for j := 0; j < len(tn.Content); j += 2 {
				if tn.Content[j].Value == "type" { typeVal = tn.Content[j+1].Value }
				if tn.Content[j].Value == "name" { nameVal = tn.Content[j+1].Value }
			}
			if typeVal == "internet_search" { continue }
			key := fmt.Sprintf("%s:%s", typeVal, nameVal)
			if !seen[key] {
				c.output.RequiredTools = append(c.output.RequiredTools, &ToolYAML{Type: typeVal, Name: nameVal})
				seen[key] = true
			}
		}
	}
}
