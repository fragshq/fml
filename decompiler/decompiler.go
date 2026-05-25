package decompiler

import (
	"fmt"
	"strings"

	"github.com/theirish81/fml/compiler"
	"github.com/theirish81/fml/parser"
	"gopkg.in/yaml.v3"
)

type sessionProp struct {
	Name   string
	Schema *compiler.JSONSchema
}

// Decompiler converts a Frags YAML plan back into FML source code.
type Decompiler struct {
	plan *compiler.PlanYAML
}

func New(plan *compiler.PlanYAML) *Decompiler {
	return &Decompiler{plan: plan}
}

// Decompile produces the FML string.
func (d *Decompiler) Decompile() (string, error) {
	if d.plan == nil {
		return "", fmt.Errorf("plan is nil")
	}
	var sb strings.Builder

	if d.plan.SystemPrompt != nil {
		d.writeBlockComment(&sb, d.plan.Comments["systemPrompt"], "")
		sb.WriteString(fmt.Sprintf("system(%q)\n\n", d.plan.SystemPrompt.Value))
	}

	if d.plan.Parameters != nil && len(d.plan.Parameters.Content) > 0 {
		d.writeBlockComment(&sb, d.plan.Comments["parameters"], "")
		for _, pNode := range d.plan.Parameters.Content {
			if err := d.writeParameter(&sb, pNode); err != nil {
				return "", err
			}
		}
		sb.WriteString("\n")
	}

	if d.plan.Vars != nil && len(d.plan.Vars.Content) > 0 {
		d.writeBlockComment(&sb, d.plan.Comments["vars"], "")
		for i := 0; i+1 < len(d.plan.Vars.Content); i += 2 {
			if err := d.writeVar(&sb, d.plan.Vars.Content[i], d.plan.Vars.Content[i+1], ""); err != nil {
				return "", err
			}
		}
		sb.WriteString("\n")
	}

	if d.plan.RequiredTools != nil && len(d.plan.RequiredTools) > 0 {
		d.writeBlockComment(&sb, d.plan.Comments["requiredTools"], "")
		for _, tool := range d.plan.RequiredTools {
			if tool.Type == "internet_search" {
				sb.WriteString("require search\n")
			} else {
				sb.WriteString(fmt.Sprintf("require %s %s\n", tool.Type, tool.Name))
			}
		}
		sb.WriteString("\n")
	}

	if d.plan.Transformers != nil && len(d.plan.Transformers.Content) > 0 {
		d.writeBlockComment(&sb, d.plan.Comments["transformers"], "")
		for _, tNode := range d.plan.Transformers.Content {
			if err := d.writeTransformer(&sb, tNode); err != nil {
				return "", err
			}
		}
		sb.WriteString("\n")
	}

	if d.plan.PreCalls != nil && len(d.plan.PreCalls.Content) > 0 {
		d.writeBlockComment(&sb, d.plan.Comments["preCalls"], "")
		for _, cNode := range d.plan.PreCalls.Content {
			if err := d.writeCall(&sb, cNode, ""); err != nil {
				return "", err
			}
		}
		sb.WriteString("\n")
	}

	if d.plan.Components != nil {
		d.writeBlockComment(&sb, d.plan.Comments["components"], "")
		sb.WriteString("components {\n")
		if len(d.plan.Components.Schemas) > 0 {
			for name, schema := range d.plan.Components.Schemas {
				sb.WriteString(fmt.Sprintf("    schema(%q) {\n", name))
				if err := d.writeSchemaFields(&sb, schema, "        "); err != nil {
					return "", err
				}
				sb.WriteString("    }")
				if schema.Description != "" {
					sb.WriteString(fmt.Sprintf(" # %s", schema.Description))
				}
				sb.WriteString("\n")
			}
		}
		if len(d.plan.Components.Prompts) > 0 {
			for name, prompt := range d.plan.Components.Prompts {
				sb.WriteString(fmt.Sprintf("    prompt(%q) {\n", name))
				sb.WriteString(fmt.Sprintf("        %q\n", prompt.Value))
				sb.WriteString("    }")
				if prompt.LineComment != "" {
					sb.WriteString(fmt.Sprintf(" # %s", prompt.LineComment))
				}
				sb.WriteString("\n")
			}
		}
		sb.WriteString("}\n\n")
	}

	// Session processing
	if d.plan.Sessions != nil && len(d.plan.Sessions.Content) > 0 {
		d.writeBlockComment(&sb, d.plan.Comments["sessions"], "")

		sessionSchemas := make(map[string][]sessionProp)
		requiredProps := make(map[string]bool)

		if d.plan.Schema != nil {
			var rootSchema compiler.JSONSchema
			if err := d.plan.Schema.Decode(&rootSchema); err != nil {
				return "", fmt.Errorf("failed to decode root schema: %w", err)
			}
			for _, r := range rootSchema.Required {
				requiredProps[r] = true
			}

			// Traverse the schema node to find properties in their original YAML order
			propsNode := d.getMapValue(d.plan.Schema, "properties")
			if propsNode != nil {
				for i := 0; i+1 < len(propsNode.Content); i += 2 {
					propName := propsNode.Content[i].Value
					valNode := propsNode.Content[i+1]
					var s compiler.JSONSchema
					if err := valNode.Decode(&s); err == nil && s.XSession != "" {
						sessionSchemas[s.XSession] = append(sessionSchemas[s.XSession], sessionProp{
							Name:   propName,
							Schema: &s,
						})
					}
				}
			}
		}

		for i := 0; i+1 < len(d.plan.Sessions.Content); i += 2 {
			keyNode := d.plan.Sessions.Content[i]
			name := keyNode.Value
			sessNode := d.plan.Sessions.Content[i+1]
			if err := d.writeSession(&sb, keyNode, sessNode, sessionSchemas[name], requiredProps); err != nil {
				return "", err
			}
			sb.WriteString("\n")
		}
	}

	if d.plan.Schema != nil {
		d.writeBlockComment(&sb, d.plan.Comments["schema"], "")
	}

	return strings.TrimSpace(sb.String()) + "\n", nil
}

func (d *Decompiler) writeParameter(sb *strings.Builder, node *yaml.Node) error {
	d.writeBlockComment(sb, node.HeadComment, "")

	nameNode := d.getMapValue(node, "name")
	if nameNode == nil {
		return fmt.Errorf("parameter node missing 'name'")
	}
	name := nameNode.Value

	schemaNode := d.getMapValue(node, "schema")
	if schemaNode == nil {
		return fmt.Errorf("parameter %q missing 'schema'", name)
	}

	var schema compiler.JSONSchema
	if err := schemaNode.Decode(&schema); err != nil {
		return fmt.Errorf("failed to decode schema for parameter %q: %w", name, err)
	}

	typ, err := d.formatType(&schema, "")
	if err != nil {
		return err
	}
	sb.WriteString(fmt.Sprintf("parameter(%q, type=%s", name, typ))

	if schema.Default != nil {
		var defNode yaml.Node
		if err := defNode.Encode(schema.Default); err != nil {
			return fmt.Errorf("failed to encode default value for parameter %q: %w", name, err)
		}
		sb.WriteString(fmt.Sprintf(", default=%s", d.formatValue(&defNode)))
	}

	if schema.Title != "" {
		sb.WriteString(fmt.Sprintf(", title=%q", schema.Title))
	}

	sb.WriteString(")")

	// 1. Check for YAML inline comment on the parameter node
	comment := ""
	if node.LineComment != "" {
		comment = strings.TrimSpace(strings.TrimPrefix(node.LineComment, "#"))
	}
	// 2. Check for YAML inline comment on the 'name' key or value
	if comment == "" {
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == "name" {
				if node.Content[i].LineComment != "" {
					comment = strings.TrimSpace(strings.TrimPrefix(node.Content[i].LineComment, "#"))
				} else if node.Content[i+1].LineComment != "" {
					comment = strings.TrimSpace(strings.TrimPrefix(node.Content[i+1].LineComment, "#"))
				}
				break
			}
		}
	}
	// 3. Check for YAML inline comment on the 'schema' key or value
	if comment == "" {
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == "schema" {
				if node.Content[i].LineComment != "" {
					comment = strings.TrimSpace(strings.TrimPrefix(node.Content[i].LineComment, "#"))
				} else if node.Content[i+1].LineComment != "" {
					comment = strings.TrimSpace(strings.TrimPrefix(node.Content[i+1].LineComment, "#"))
				}
				break
			}
		}
	}
	// 4. Check for 'description' at the parameter level (sibling of 'name')
	if comment == "" {
		if dNode := d.getMapValue(node, "description"); dNode != nil {
			comment = dNode.Value
		}
	}
	// 5. Check for 'description' inside the schema
	if comment == "" && schema.Description != "" {
		comment = schema.Description
	}

	if comment != "" {
		sb.WriteString(fmt.Sprintf(" # %s", comment))
	}
	sb.WriteString("\n")
	return nil
}

func (d *Decompiler) writeTransformer(sb *strings.Builder, node *yaml.Node) error {
	d.writeBlockComment(sb, node.HeadComment, "")
	nameNode := d.getMapValue(node, "name")
	if nameNode == nil {
		return fmt.Errorf("transformer node missing 'name'")
	}
	name := nameNode.Value
	sb.WriteString(fmt.Sprintf("transformer(%q) {", name))
	if node.LineComment != "" {
		sb.WriteString(fmt.Sprintf(" # %s", node.LineComment))
	}
	sb.WriteString("\n")

	if v := d.getMapValue(node, "onFunctionOutput"); v != nil {
		sb.WriteString(fmt.Sprintf("    onFunctionOutput = %q\n", v.Value))
	}
	if v := d.getMapValue(node, "onFunctionInput"); v != nil {
		sb.WriteString(fmt.Sprintf("    onFunctionInput = %q\n", v.Value))
	}
	if v := d.getMapValue(node, "onResource"); v != nil {
		sb.WriteString(fmt.Sprintf("    onResource = %q\n", v.Value))
	}
	if v := d.getMapValue(node, "jmesPath"); v != nil {
		sb.WriteString(fmt.Sprintf("    jmesPath = %q\n", v.Value))
	}
	if v := d.getMapValue(node, "parser"); v != nil {
		val := v.Value
		if val == "json" || val == "csv" {
			sb.WriteString(fmt.Sprintf("    parser = %s\n", val))
		} else {
			sb.WriteString(fmt.Sprintf("    parser = %q\n", val))
		}
	}
	if v := d.getMapValue(node, "code"); v != nil {
		sb.WriteString(fmt.Sprintf("    code( %s )\n", v.Value))
	}
	sb.WriteString("}\n")
	return nil
}

func (d *Decompiler) writeSession(sb *strings.Builder, keyNode *yaml.Node, sessNode *yaml.Node, schemas []sessionProp, requiredProps map[string]bool) error {
	d.writeBlockComment(sb, keyNode.HeadComment, "")
	name := keyNode.Value
	sb.WriteString(fmt.Sprintf("session(%q", name))

	if len(schemas) == 1 {
		if schemas[0].Name != name {
			sb.WriteString(fmt.Sprintf(", target=%q", schemas[0].Name))
		}
	}

	deps := d.getMapValue(sessNode, "dependsOn")
	if deps != nil {
		for _, depNode := range deps.Content {
			s := d.getMapValue(depNode, "session")
			if s != nil {
				sb.WriteString(fmt.Sprintf(", after=%q", s.Value))
			}
			e := d.getMapValue(depNode, "expression")
			if e != nil {
				sb.WriteString(fmt.Sprintf(", expect=%s", e.Value))
			}
		}
	}

	it := d.getMapValue(sessNode, "iterateOn")
	if it != nil {
		sb.WriteString(fmt.Sprintf(", iterate=%s", it.Value))
	}
	sb.WriteString(") {")
	if keyNode.LineComment != "" {
		sb.WriteString(fmt.Sprintf(" # %s", keyNode.LineComment))
	}
	sb.WriteString("\n")

	// Vars
	varsKey := d.getMapKey(sessNode, "vars")
	vars := d.getMapValue(sessNode, "vars")
	if vars != nil {
		d.writeBlockComment(sb, varsKey.HeadComment, "    ")
		for i := 0; i+1 < len(vars.Content); i += 2 {
			if err := d.writeVar(sb, vars.Content[i], vars.Content[i+1], "    "); err != nil {
				return err
			}
		}
	}

	// Tools
	toolsKey := d.getMapKey(sessNode, "tools")
	tools := d.getMapValue(sessNode, "tools")
	if tools != nil {
		d.writeBlockComment(sb, toolsKey.HeadComment, "    ")
		for _, tNode := range tools.Content {
			typNode := d.getMapValue(tNode, "type")
			if typNode == nil {
				continue
			}
			typ := typNode.Value
			if typ == "internet_search" {
				d.writeBlockComment(sb, tNode.HeadComment, "    ")
				sb.WriteString("    use search\n")
			} else {
				nameNode := d.getMapValue(tNode, "name")
				if nameNode == nil {
					continue
				}
				name := nameNode.Value

				alNode := d.getMapValue(tNode, "allowlist")
				d.writeBlockComment(sb, tNode.HeadComment, "    ")
				if alNode != nil && alNode.Kind == yaml.SequenceNode && len(alNode.Content) > 0 {
					sb.WriteString(fmt.Sprintf("    use %s %s {\n", typ, name))
					sb.WriteString("        allowlist = [")
					for j, item := range alNode.Content {
						if j > 0 {
							sb.WriteString(", ")
						}
						sb.WriteString(fmt.Sprintf("%q", item.Value))
					}
					sb.WriteString("]\n")
					sb.WriteString("    }\n")
				} else {
					sb.WriteString(fmt.Sprintf("    use %s %s\n", typ, name))
				}
			}
		}
	}

	// Resources
	resourcesKey := d.getMapKey(sessNode, "resources")
	resources := d.getMapValue(sessNode, "resources")
	if resources != nil {
		d.writeBlockComment(sb, resourcesKey.HeadComment, "    ")
		for _, rNode := range resources.Content {
			idNode := d.getMapValue(rNode, "identifier")
			if idNode == nil {
				continue
			}
			d.writeBlockComment(sb, rNode.HeadComment, "    ")
			sb.WriteString(fmt.Sprintf("    resource %q", idNode.Value))

			in := d.getMapValue(rNode, "in")
			target := d.getMapValue(rNode, "var")
			if target != nil {
				if in != nil && in.Value != "vars" {
					sb.WriteString(fmt.Sprintf(" -> %s:%s", in.Value, target.Value))
				} else {
					sb.WriteString(fmt.Sprintf(" -> %s", target.Value))
				}
			}
			sb.WriteString("\n")
		}
	}

	// Calls
	callsKey := d.getMapKey(sessNode, "preCalls")
	calls := d.getMapValue(sessNode, "preCalls")
	if calls != nil {
		d.writeBlockComment(sb, callsKey.HeadComment, "    ")
		for _, cNode := range calls.Content {
			if err := d.writeCall(sb, cNode, "    "); err != nil {
				return err
			}
		}
	}

	// Context
	ctxKey := d.getMapKey(sessNode, "context")
	ctx := d.getMapValue(sessNode, "context")
	if ctx != nil {
		d.writeBlockComment(sb, ctxKey.HeadComment, "    ")
		sb.WriteString(fmt.Sprintf("    context %s\n", d.formatValue(ctx)))
	}

	// Prompts
	prePromptKey := d.getMapKey(sessNode, "prePrompt")
	prePrompt := d.getMapValue(sessNode, "prePrompt")
	if prePrompt != nil {
		d.writeBlockComment(sb, prePromptKey.HeadComment, "    ")
		if prePrompt.Kind == yaml.SequenceNode {
			for _, p := range prePrompt.Content {
				d.writePromptItem(sb, p.Value, "+")
			}
		} else {
			d.writePromptItem(sb, prePrompt.Value, "+")
		}
	}
	promptKey := d.getMapKey(sessNode, "prompt")
	prompt := d.getMapValue(sessNode, "prompt")
	if prompt != nil {
		d.writeBlockComment(sb, promptKey.HeadComment, "    ")
		d.writePromptItem(sb, prompt.Value, "-")
	}

	// Schema
	if len(schemas) > 0 {
		isIterated := it != nil
		for _, sp := range schemas {
			propName := sp.Name
			s := sp.Schema
			if isIterated && s.Type == parser.TypeArray && s.Items != nil {
				s = s.Items
			}

			opt := ""
			if !requiredProps[propName] {
				opt = "?"
			}

			if s.Type == parser.TypeObject && len(s.Properties) > 0 {
				sb.WriteString(fmt.Sprintf("    schema%s {\n", opt))
				if err := d.writeSchemaFields(sb, s, "        "); err != nil {
					return err
				}
				sb.WriteString("    }")
				if s.Description != "" {
					sb.WriteString(fmt.Sprintf(" # %s", s.Description))
				}
				sb.WriteString("\n")
			} else {
				// Avoid writing default session schemas (previously empty object, now string)
				if (s.Type == parser.TypeObject || s.Type == parser.TypeString) && len(s.Properties) == 0 && s.Ref == "" && len(s.Enum) == 0 && s.Description == "" {
					continue
				}

				ft, err := d.formatType(s, "    ")
				if err != nil {
					return err
				}
				sb.WriteString(fmt.Sprintf("    schema%s %s", opt, ft))
				if s.Description != "" {
					sb.WriteString(fmt.Sprintf(" # %s", s.Description))
				}
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("}\n")
	return nil
}

func (d *Decompiler) writeVar(sb *strings.Builder, keyNode *yaml.Node, valNode *yaml.Node, indent string) error {
	d.writeBlockComment(sb, keyNode.HeadComment, indent)
	sb.WriteString(fmt.Sprintf("%sset %s = %s", indent, keyNode.Value, d.formatValue(valNode)))
	if keyNode.LineComment != "" {
		sb.WriteString(fmt.Sprintf(" # %s", strings.TrimSpace(strings.TrimPrefix(keyNode.LineComment, "#"))))
	}
	sb.WriteString("\n")
	return nil
}

func (d *Decompiler) writeCall(sb *strings.Builder, cNode *yaml.Node, indent string) error {
	d.writeBlockComment(sb, cNode.HeadComment, indent)
	cNameNode := d.getMapValue(cNode, "name")
	if cNameNode == nil {
		return fmt.Errorf("call node missing 'name'")
	}
	cName := cNameNode.Value
	sb.WriteString(fmt.Sprintf("%scall(%q)", indent, cName))

	in := d.getMapValue(cNode, "in")
	target := d.getMapValue(cNode, "var")
	if target != nil {
		if in != nil && in.Value != "vars" && in.Value != "ai" {
			sb.WriteString(fmt.Sprintf(" -> %s:%s", in.Value, target.Value))
		} else {
			sb.WriteString(fmt.Sprintf(" -> %s", target.Value))
		}
	}

	args := d.getMapValue(cNode, "args")
	code := d.getMapValue(cNode, "code")

	if args != nil || code != nil {
		sb.WriteString(" {\n")
		if args != nil {
			for i := 0; i+1 < len(args.Content); i += 2 {
				sb.WriteString(fmt.Sprintf("%s    %s = %s\n", indent, d.formatKey(args.Content[i].Value), d.formatValue(args.Content[i+1])))
			}
		}
		if code != nil {
			sb.WriteString(fmt.Sprintf("%s    code( %s )\n", indent, code.Value))
		}
		sb.WriteString(fmt.Sprintf("%s}\n", indent))
	} else {
		sb.WriteString("\n")
	}
	return nil
}

func (d *Decompiler) writePromptItem(sb *strings.Builder, text string, prefix string) {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 {
		return
	}
	sb.WriteString(fmt.Sprintf("    %s %s\n", prefix, strings.TrimSpace(lines[0])))
	for _, line := range lines[1:] {
		sb.WriteString(fmt.Sprintf("      %s\n", strings.TrimSpace(line)))
	}
}

func (d *Decompiler) writeSchemaFields(sb *strings.Builder, s *compiler.JSONSchema, indent string) error {
	for name, prop := range s.Properties {
		sb.WriteString(fmt.Sprintf("%s%s", indent, d.formatKey(name)))
		isRequired := false
		for _, r := range s.Required {
			if r == name {
				isRequired = true
				break
			}
		}
		if !isRequired {
			sb.WriteString("?")
		}
		ft, err := d.formatType(prop, indent)
		if err != nil {
			return err
		}
		sb.WriteString(fmt.Sprintf(": %s", ft))
		if prop.Description != "" && !strings.Contains(ft, "\n") {
			sb.WriteString(fmt.Sprintf(" # %s", prop.Description))
		}
		sb.WriteString("\n")
	}
	return nil
}

func (d *Decompiler) formatType(s *compiler.JSONSchema, indent string) (string, error) {
	if s == nil {
		return "any", nil
	}
	if s.Ref != "" {
		return "$" + strings.TrimPrefix(s.Ref, "#/components/schemas/"), nil
	}
	if len(s.Enum) > 0 {
		vals := make([]string, len(s.Enum))
		for i, v := range s.Enum {
			vals[i] = fmt.Sprintf("%v", v)
		}
		return strings.Join(vals, "|"), nil
	}
	switch s.Type {
	case parser.TypeInteger:
		return "int", nil
	case parser.TypeNumber:
		return "float", nil
	case parser.TypeArray:
		if s.Items != nil {
			ft, err := d.formatType(s.Items, indent)
			if err != nil {
				return "", err
			}
			if strings.Contains(ft, "\n") {
				return ft + "[]", nil
			}
			return ft + "[]", nil
		}
		return "any[]", nil
	case parser.TypeObject:
		if len(s.Properties) == 0 {
			return "any", nil
		}

		var sb strings.Builder
		sb.WriteString("{\n")
		newIndent := indent + "    "
		for name, prop := range s.Properties {
			sb.WriteString(newIndent + name)
			isRequired := false
			for _, r := range s.Required {
				if r == name {
					isRequired = true
					break
				}
			}
			if !isRequired {
				sb.WriteString("?")
			}
			ft, err := d.formatType(prop, newIndent)
			if err != nil {
				return "", err
			}
			sb.WriteString(": " + ft)
			if prop.Description != "" {
				sb.WriteString(fmt.Sprintf(" # %s", prop.Description))
			}
			sb.WriteString("\n")
		}
		sb.WriteString(indent + "}")
		return sb.String(), nil
	case "":
		return "any", nil
	default:
		return s.Type, nil
	}
}

func (d *Decompiler) formatKey(key string) string {
	// If it matches IDENTIFIER [a-zA-Z_][a-zA-Z0-9_-]*, no quotes needed.
	// Otherwise, use %q.
	if len(key) == 0 {
		return `""`
	}
	for i, r := range key {
		if i == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
				return fmt.Sprintf("%q", key)
			}
		} else {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
				return fmt.Sprintf("%q", key)
			}
		}
	}
	return key
}

func (d *Decompiler) formatValue(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	if n.Kind == yaml.ScalarNode {
		if strings.HasPrefix(n.Value, "$(") && strings.HasSuffix(n.Value, ")") {
			return n.Value
		}
		if n.Tag == "!!str" {
			return fmt.Sprintf("%q", n.Value)
		}
		if n.Tag == "!!bool" || n.Tag == "!!int" || n.Tag == "!!float" {
			return n.Value
		}
		return n.Value
	}
	if n.Kind == yaml.MappingNode {
		var sb strings.Builder
		sb.WriteString("{")
		for i := 0; i+1 < len(n.Content); i += 2 {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%s: %s", d.formatKey(n.Content[i].Value), d.formatValue(n.Content[i+1])))
		}
		sb.WriteString("}")
		return sb.String()
	}
	if n.Kind == yaml.SequenceNode {
		var sb strings.Builder
		sb.WriteString("[")
		for i, item := range n.Content {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(d.formatValue(item))
		}
		sb.WriteString("]")
		return sb.String()
	}
	return "null"
}

func (d *Decompiler) getMapValue(node *yaml.Node, key string) *yaml.Node {
	_, v := d.getMapEntry(node, key)
	return v
}

func (d *Decompiler) getMapKey(node *yaml.Node, key string) *yaml.Node {
	k, _ := d.getMapEntry(node, key)
	return k
}

func (d *Decompiler) getMapEntry(node *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i], node.Content[i+1]
		}
	}
	return nil, nil
}

func (d *Decompiler) writeBlockComment(sb *strings.Builder, comment string, indent string) {
	if comment == "" {
		return
	}
	lines := strings.Split(comment, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "#"))
		sb.WriteString(fmt.Sprintf("%s# %s\n", indent, trimmed))
	}
}
