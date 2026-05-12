package decompiler

import (
	"fmt"
	"strings"

	"github.com/theirish/fml/compiler"
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
	var sb strings.Builder

	if d.plan.SystemPrompt != nil {
		sb.WriteString(fmt.Sprintf("system(%q)\n\n", d.plan.SystemPrompt.Value))
	}

	if d.plan.Parameters != nil && len(d.plan.Parameters.Content) > 0 {
		for _, pNode := range d.plan.Parameters.Content {
			d.writeParameter(&sb, pNode)
		}
		sb.WriteString("\n")
	}

	if d.plan.Vars != nil && len(d.plan.Vars.Content) > 0 {
		for i := 0; i < len(d.plan.Vars.Content); i += 2 {
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
			d.writeTransformer(&sb, tNode)
		}
		sb.WriteString("\n")
	}

	if d.plan.Components != nil {
		sb.WriteString("components {\n")
		if len(d.plan.Components.Schemas) > 0 {
			for name, schema := range d.plan.Components.Schemas {
				sb.WriteString(fmt.Sprintf("    schema(%q) {\n", name))
				d.writeSchemaFields(&sb, schema, "        ")
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
			d.plan.Schema.Decode(&rootSchema)
			for _, r := range rootSchema.Required {
				requiredProps[r] = true
			}
			for propName, s := range rootSchema.Properties {
				if s.XSession != "" {
					if _, ok := sessionSchemas[s.XSession]; !ok {
						sessionSchemas[s.XSession] = make(map[string]*compiler.JSONSchema)
					}
					sessionSchemas[s.XSession][propName] = s
				}
			}
		}

		for i := 0; i < len(d.plan.Sessions.Content); i += 2 {
			name := d.plan.Sessions.Content[i].Value
			sessNode := d.plan.Sessions.Content[i+1]
			d.writeSession(&sb, name, sessNode, sessionSchemas[name], requiredProps)
			sb.WriteString("\n")
		}
	}

	return strings.TrimSpace(sb.String()) + "\n", nil
}

func (d *Decompiler) writeParameter(sb *strings.Builder, node *yaml.Node) {
	if node.HeadComment != "" {
		for _, line := range strings.Split(node.HeadComment, "\n") {
			sb.WriteString(fmt.Sprintf("# %s\n", line))
		}
	}

	name := d.getMapValue(node, "name").Value
	schemaNode := d.getMapValue(node, "schema")
	var schema compiler.JSONSchema
	schemaNode.Decode(&schema)

	sb.WriteString(fmt.Sprintf("parameter(%q, type=%s", name, d.formatType(&schema)))

	if schema.Default != nil {
		// We need to format the default value. Since it's interface{}, we can encode it to a node first.
		var defNode yaml.Node
		defNode.Encode(schema.Default)
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
}

func (d *Decompiler) writeTransformer(sb *strings.Builder, node *yaml.Node) {
	name := d.getMapValue(node, "name").Value
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
}

func (d *Decompiler) writeSession(sb *strings.Builder, name string, sessNode *yaml.Node, schemas map[string]*compiler.JSONSchema, requiredProps map[string]bool) {
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
		for i := 0; i < len(vars.Content); i += 2 {
			sb.WriteString(fmt.Sprintf("    set %s = %s\n", vars.Content[i].Value, d.formatValue(vars.Content[i+1])))
		}
	}

	// Tools
	tools := d.getMapValue(sessNode, "tools")
	if tools != nil {
		for _, tNode := range tools.Content {
			typ := d.getMapValue(tNode, "type").Value
			if typ == "internet_search" {
				sb.WriteString("    use search\n")
			} else {
				name := d.getMapValue(tNode, "name").Value
				sb.WriteString(fmt.Sprintf("    use %s %s\n", typ, name))
			}
		}
	}

	// Calls
	calls := d.getMapValue(sessNode, "preCalls")
	if calls != nil {
		for _, cNode := range calls.Content {
			cName := d.getMapValue(cNode, "name").Value
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
				for i := 0; i < len(args.Content); i += 2 {
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
			if isIterated && s.Type == "array" && s.Items != nil {
				s = s.Items
			}

			opt := ""
			if !requiredProps[propName] {
				opt = "?"
			}

			if s.Type == "object" && len(s.Properties) > 0 {
				sb.WriteString(fmt.Sprintf("    schema%s {\n", opt))
				d.writeSchemaFields(sb, s, "        ")
				sb.WriteString("    }\n")
			} else {
				sb.WriteString(fmt.Sprintf("    schema%s %s\n", opt, d.formatType(s)))
			}
		}
	}

	sb.WriteString("}\n")
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

func (d *Decompiler) writeSchemaFields(sb *strings.Builder, s *compiler.JSONSchema, indent string) {
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
		sb.WriteString(fmt.Sprintf(": %s\n", d.formatType(prop)))
	}
}

func (d *Decompiler) formatType(s *compiler.JSONSchema) string {
	if s.Ref != "" {
		return "$" + strings.TrimPrefix(s.Ref, "#/components/schemas/")
	}
	if len(s.Enum) > 0 {
		vals := make([]string, len(s.Enum))
		for i, v := range s.Enum {
			vals[i] = fmt.Sprintf("%v", v)
		}
		return strings.Join(vals, "|")
	}
	switch s.Type {
	case "integer":
		return "int"
	case "number":
		return "float"
	case "array":
		if s.Items != nil {
			return d.formatType(s.Items) + "[]"
		}
		return "any[]"
	case "object":
		if len(s.Properties) == 0 {
			return "any"
		}
		var sb strings.Builder
		sb.WriteString("{")
		first := true
		for name, prop := range s.Properties {
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(name + ": " + d.formatType(prop))
			first = false
		}
		sb.WriteString("}")
		return sb.String()
	case "":
		return "any"
	default:
		return s.Type
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
		for i := 0; i < len(n.Content); i += 2 {
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
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
