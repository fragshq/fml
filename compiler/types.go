package compiler

import "gopkg.in/yaml.v3"

// PlanYAML represents the finalized Frags plan file structure with granular comment support.
type PlanYAML struct {
	SystemPrompt  *yaml.Node      `yaml:"systemPrompt,omitempty"`
	Parameters    *yaml.Node      `yaml:"parameters,omitempty"` // SequenceNode
	Vars          *yaml.Node      `yaml:"vars,omitempty"`       // MappingNode
	RequiredTools []*ToolYAML     `yaml:"requiredTools,omitempty"`
	Transformers  *yaml.Node      `yaml:"transformers,omitempty"` // SequenceNode
	PreCalls      *yaml.Node      `yaml:"preCalls,omitempty"`     // SequenceNode
	Sessions      *yaml.Node      `yaml:"sessions,omitempty"`     // MappingNode
	Schema        *yaml.Node      `yaml:"schema,omitempty"`
	Components    *ComponentsYAML `yaml:"components,omitempty"`

	// Comments captures HeadComments for top-level keys
	Comments map[string]string `yaml:"-"`
}

func (p *PlanYAML) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return nil
	}
	p.Comments = make(map[string]string)
	for i := 0; i+1 < len(value.Content); i += 2 {
		keyNode := value.Content[i]
		key := keyNode.Value
		val := value.Content[i+1]

		if keyNode.HeadComment != "" {
			p.Comments[key] = keyNode.HeadComment
		}

		switch key {
		case "systemPrompt":
			p.SystemPrompt = val
		case "parameters":
			p.Parameters = val
		case "vars":
			p.Vars = val
		case "requiredTools":
			if err := val.Decode(&p.RequiredTools); err != nil {
				return err
			}
		case "transformers":
			p.Transformers = val
		case "preCalls":
			p.PreCalls = val
		case "sessions":
			p.Sessions = val
		case "schema":
			p.Schema = val
		case "components":
			p.Components = &ComponentsYAML{}
			if err := val.Decode(p.Components); err != nil {
				return err
			}
		}
	}
	return nil
}

type ParameterYAML struct {
	Name   string      `yaml:"name"`
	Schema *JSONSchema `yaml:"schema"`
}

type ToolYAML struct {
	Type      string   `yaml:"type"`
	Name      string   `yaml:"name,omitempty"`
	Allowlist []string `yaml:"allowlist,omitempty"`
}

type TransformerYAML struct {
	Name             string `yaml:"name"`
	OnFunctionOutput string `yaml:"onFunctionOutput,omitempty"`
	OnFunctionInput  string `yaml:"onFunctionInput,omitempty"`
	OnResource       string `yaml:"onResource,omitempty"`
	JMESPath         string `yaml:"jmesPath"`
	Parser           string `yaml:"parser,omitempty"`
	Code             string `yaml:"code,omitempty"`
}

type CallYAML struct {
	Name string                 `yaml:"name"`
	Args map[string]interface{} `yaml:"args,omitempty"`
	Code string                 `yaml:"code,omitempty"`
	In   string                 `yaml:"in,omitempty"`
	Var  string                 `yaml:"var,omitempty"`
}

type ResourceYAML struct {
	Identifier string `yaml:"identifier"`
	In         string `yaml:"in,omitempty"`
	Var        string `yaml:"var,omitempty"`
}

type SessionYAML struct {
	DependsOn []*DependsOnYAML `yaml:"dependsOn,omitempty"`
	IterateOn string           `yaml:"iterateOn,omitempty"`
	Vars      *yaml.Node       `yaml:"vars,omitempty"` // MappingNode
	Tools     []*ToolYAML      `yaml:"tools,omitempty"`
	PreCalls  *yaml.Node       `yaml:"preCalls,omitempty"` // SequenceNode
	Context   *yaml.Node       `yaml:"context,omitempty"`
	PrePrompt *yaml.Node       `yaml:"prePrompt,omitempty"`
	Prompt    *yaml.Node       `yaml:"prompt,omitempty"`
}

type DependsOnYAML struct {
	Session    string `yaml:"session,omitempty"`
	Expression string `yaml:"expression,omitempty"`
}

type ComponentsYAML struct {
	Schemas map[string]*JSONSchema `yaml:"schemas,omitempty"`
	Prompts map[string]*yaml.Node  `yaml:"prompts,omitempty"`
}

func (c *ComponentsYAML) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		key := value.Content[i].Value
		val := value.Content[i+1]
		switch key {
		case "schemas":
			if err := val.Decode(&c.Schemas); err != nil {
				return err
			}
		case "prompts":
			c.Prompts = make(map[string]*yaml.Node)
			if val.Kind == yaml.MappingNode {
				for j := 0; j < len(val.Content); j += 2 {
					c.Prompts[val.Content[j].Value] = val.Content[j+1]
				}
			}
		}
	}
	return nil
}

// JSONSchema node with comment support for fields that aren't description-driven.
type JSONSchema struct {
	Type        string                 `yaml:"type,omitempty"`
	Title       string                 `yaml:"title,omitempty"`
	Description string                 `yaml:"description,omitempty"`
	Default     interface{}            `yaml:"default,omitempty"`
	Properties  map[string]*JSONSchema `yaml:"properties,omitempty"`
	Required    []string               `yaml:"required,omitempty"`
	Items       *JSONSchema            `yaml:"items,omitempty"`
	Enum        []interface{}          `yaml:"enum,omitempty"`
	Ref         string                 `yaml:"$ref,omitempty"`
	XSession    string                 `yaml:"x-session,omitempty"`
	Extensions  map[string]interface{} `yaml:",inline"`
}
