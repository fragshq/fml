package decompiler

import (
	"fmt"
	"strings"

	"github.com/theirish/fml/compiler"
	"github.com/theirish/fml/parser"
	"gopkg.in/yaml.v3"
)

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
		sb.WriteString(fmt.Sprintf("system(%q)\n\n", d.plan.SystemPrompt.Value))
	}

	if d.plan.Parameters != nil && len(d.plan.Parameters.Content) > 0 {
		for _, pNode := range d.plan.Parameters.Content {
			if err := d.writeParameter(&sb, pNode); err != nil {
				return "", err
			}
		}
		sb.WriteString("\n")
	}

	if d.plan.Vars != nil && len(d.plan.Vars.Content) > 0 {
		for i := 0; i+1 < len(d.plan.Vars.Content); i += 2 {
			key := d.plan.Vars.Content[i].Value
			valNode := d.plan.Vars.Content[i+1]
			sb.WriteString(fmt.Sprintf("set %s = %s\n", key, d.formatValue(valNode)))
		}
		sb.WriteString("\n")
	}

	if d.plan.RequiredTools != nil && len(d.plan.RequiredTools) > 0 {
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
		for _, tNode := range d.plan.Transformers.Content {
			if err := d.writeTransformer(&sb, tNode); err != nil {
				return "", err
			}
		}
		sb.WriteString("\n")
	}

	if d.plan.Components != nil {
		sb.WriteString("components {\n")
		if len(d.plan.Components.Schemas) > 0 {
			for name, schema := range d.plan.Components.Schemas {
				sb.WriteString(fmt.Sprintf("    schema(%q) {\n", name))
				if err := d.writeSchemaFields(&sb, schema, "        "); err != nil {
					return "", err
				}
				sb.WriteString("    }\n")
			}
		}
		if len(d.plan.Components.Prompts) > 0 {
			for name, prompt := range d.plan.Components.Prompts {
				sb.WriteString(fmt.Sprintf("    prompt(%q) {\n", name))
				sb.WriteString(fmt.Sprintf("        %q\n", prompt.Value))
				sb.WriteString("    }\n")
			}
		}
		sb.WriteString("}\n\n")
	}

	// Session processing
	if d.plan.Sessions != nil && len(d.plan.Sessions.Content) > 0 {
		sessionSchemas := make(map[string]map[string]*compiler.JSONSchema)
		requiredProps := make(map[string]bool)
		if d.plan.Schema != nil {
			var rootSchema compiler.JSONSchema
			if err := d.plan.Schema.Decode(&rootSchema); err != nil {
				return "", fmt.Errorf("failed to decode root schema: %w", err)
			}
			for _, r := range rootSchema.Required {
				requiredProps[r] = true
			}
			for propName, s := range rootSchema.Properties {
				if s != nil && s.XSession != "" {
					if _, ok := sessionSchemas[s.XSession]; !ok {
						sessionSchemas[s.XSession] = make(map[string]*compiler.JSONSchema)
					}
					sessionSchemas[s.XSession][propName] = s
				}
			}
		}

		for i := 0; i+1 < len(d.plan.Sessions.Content); i += 2 {
			name := d.plan.Sessions.Content[i].Value
			sessNode := d.plan.Sessions.Content[i+1]
			if err := d.writeSession(&sb, name, sessNode, sessionSchemas[name], requiredProps); err != nil {
				return "", err
			}
			sb.WriteString("\n")
		}
	}

	return strings.TrimSpace(sb.String()) + "\n", nil
}

func (d *Decompiler) writeParameter(sb *strings.Builder, node *yaml.Node) error {
	if node.HeadComment != "" {
		for _, line := range strings.Split(node.HeadComment, "\n") {
			sb.WriteString(fmt.Sprintf("# %s\n", line))
		}
	}

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

	typ, err := d.formatType(&schema)
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

	if node.LineComment != "" {
		sb.WriteString(fmt.Sprintf(" # %s", node.LineComment))
	} else if schema.Description != "" {
		sb.WriteString(fmt.Sprintf(" # %s", schema.Description))
	}
	sb.WriteString("\n")
	return nil
}

func (d *Decompiler) writeTransformer(sb *strings.Builder, node *yaml.Node) error {
	nameNode := d.getMapValue(node, "name")
	if nameNode == nil {
		return fmt.Errorf("transformer node missing 'name'")
	}
	name := nameNode.Value
	sb.WriteString(fmt.Sprintf("transformer(%q) {\n", name))

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

func (d *Decompiler) writeSession(sb *strings.Builder, name string, sessNode *yaml.Node, schemas map[string]*compiler.JSONSchema, requiredProps map[string]bool) error {
	sb.WriteString(fmt.Sprintf("session(%q", name))

	if len(schemas) == 1 {
		for propName := range schemas {
			if propName != name {
				sb.WriteString(fmt.Sprintf(", target=%q", propName))
			}
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
	sb.WriteString(") {\n")

	// Vars
	vars := d.getMapValue(sessNode, "vars")
	if vars != nil {
		for i := 0; i+1 < len(vars.Content); i += 2 {
			sb.WriteString(fmt.Sprintf("    set %s = %s\n", vars.Content[i].Value, d.formatValue(vars.Content[i+1])))
		}
	}

	// Tools
	tools := d.getMapValue(sessNode, "tools")
	if tools != nil {
		for _, tNode := range tools.Content {
			typNode := d.getMapValue(tNode, "type")
			if typNode == nil {
				continue
			}
			typ := typNode.Value
			if typ == "internet_search" {
				sb.WriteString("    use search\n")
			} else {
				nameNode := d.getMapValue(tNode, "name")
				if nameNode == nil {
					continue
				}
				name := nameNode.Value

				alNode := d.getMapValue(tNode, "allowlist")
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

	// Calls
	calls := d.getMapValue(sessNode, "preCalls")
	if calls != nil {
		for _, cNode := range calls.Content {
			cNameNode := d.getMapValue(cNode, "name")
			if cNameNode == nil {
				continue
			}
			cName := cNameNode.Value
			sb.WriteString(fmt.Sprintf("    call(%q)", cName))

			in := d.getMapValue(cNode, "in")
			target := d.getMapValue(cNode, "var")
			if target != nil {
				if in != nil && in.Value != "vars" && in.Value != "ai" {
					sb.WriteString(fmt.Sprintf(" -> %s:%s", in.Value, target.Value))
				} else {
					sb.WriteString(fmt.Sprintf(" -> %s", target.Value))
				}
			}
			sb.WriteString(" {\n")

			args := d.getMapValue(cNode, "args")
			if args != nil {
				for i := 0; i+1 < len(args.Content); i += 2 {
					sb.WriteString(fmt.Sprintf("        %s = %s\n", args.Content[i].Value, d.formatValue(args.Content[i+1])))
				}
			}

			code := d.getMapValue(cNode, "code")
			if code != nil {
				sb.WriteString(fmt.Sprintf("        code( %s )\n", code.Value))
			}
			sb.WriteString("    }\n")
		}
	}

	// Context
	ctx := d.getMapValue(sessNode, "context")
	if ctx != nil {
		sb.WriteString(fmt.Sprintf("    context %s\n", d.formatValue(ctx)))
	}

	// Prompts
	prePrompt := d.getMapValue(sessNode, "prePrompt")
	if prePrompt != nil {
		if prePrompt.Kind == yaml.SequenceNode {
			for _, p := range prePrompt.Content {
				d.writePromptItem(sb, p.Value, "+")
			}
		} else {
			d.writePromptItem(sb, prePrompt.Value, "+")
		}
	}
	prompt := d.getMapValue(sessNode, "prompt")
	if prompt != nil {
		d.writePromptItem(sb, prompt.Value, "-")
	}

	// Schema
	if len(schemas) > 0 {
		isIterated := it != nil
		for propName, s := range schemas {
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
				sb.WriteString("    }\n")
			} else {
				// Avoid writing 'schema any' for default empty session schemas
				if s.Type == parser.TypeObject && len(s.Properties) == 0 && s.Ref == "" && len(s.Enum) == 0 && s.Description == "" {
					continue
				}

				ft, err := d.formatType(s)
				if err != nil {
					return err
				}
				sb.WriteString(fmt.Sprintf("    schema%s %s\n", opt, ft))
			}
		}
	}

	sb.WriteString("}\n")
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
		sb.WriteString(fmt.Sprintf("%s%s", indent, name))
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
		ft, err := d.formatType(prop)
		if err != nil {
			return err
		}
		sb.WriteString(fmt.Sprintf(": %s\n", ft))
	}
	return nil
}

func (d *Decompiler) formatType(s *compiler.JSONSchema) (string, error) {
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
			ft, err := d.formatType(s.Items)
			if err != nil {
				return "", err
			}
			return ft + "[]", nil
		}
		return "any[]", nil
	case parser.TypeObject:
		if len(s.Properties) == 0 {
			return "any", nil
		}
		var sb strings.Builder
		sb.WriteString("{")
		first := true
		for name, prop := range s.Properties {
			if !first {
				sb.WriteString(", ")
			}
			ft, err := d.formatType(prop)
			if err != nil {
				return "", err
			}
			sb.WriteString(name + ": " + ft)
			first = false
		}
		sb.WriteString("}")
		return sb.String(), nil
	case "":
		return "any", nil
	default:
		return s.Type, nil
	}
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
			sb.WriteString(fmt.Sprintf("%s: %s", n.Content[i].Value, d.formatValue(n.Content[i+1])))
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
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
