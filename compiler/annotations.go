package compiler

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// GroupComments splits comment lines into structured annotations and regular description lines.
func GroupComments(comments []string) (annotations []string, descriptions []string) {
	var currentAnno strings.Builder
	braceDepth := 0
	bracketDepth := 0
	inAnno := false

	for _, rawLine := range comments {
		line := rawLine
		if strings.HasPrefix(line, "#") {
			line = line[1:]
		}
		trimmed := strings.TrimSpace(line)

		if inAnno {
			currentAnno.WriteString("\n")
			currentAnno.WriteString(line)

			for _, r := range trimmed {
				if r == '{' {
					braceDepth++
				} else if r == '}' {
					braceDepth--
				} else if r == '[' {
					bracketDepth++
				} else if r == ']' {
					bracketDepth--
				}
			}

			if braceDepth <= 0 && bracketDepth <= 0 {
				annotations = append(annotations, currentAnno.String())
				currentAnno.Reset()
				inAnno = false
			}
		} else {
			if strings.HasPrefix(trimmed, "@") {
				inAnno = true
				currentAnno.WriteString(line)

				for _, r := range trimmed {
					if r == '{' {
						braceDepth++
					} else if r == '}' {
						braceDepth--
					} else if r == '[' {
						bracketDepth++
					} else if r == ']' {
						bracketDepth--
					}
				}

				if braceDepth <= 0 && bracketDepth <= 0 {
					annotations = append(annotations, currentAnno.String())
					currentAnno.Reset()
					inAnno = false
				}
			} else {
				descriptions = append(descriptions, rawLine)
			}
		}
	}

	if inAnno && currentAnno.Len() > 0 {
		annotations = append(annotations, currentAnno.String())
	}

	return annotations, descriptions
}

// ParseAnnotations parses raw annotation strings and returns a map of extensions.
func ParseAnnotations(comments []string) (map[string]interface{}, []string) {
	annos, descs := GroupComments(comments)
	if len(annos) == 0 {
		return nil, descs
	}

	extensions := make(map[string]interface{})
	for _, anno := range annos {
		trimmed := strings.TrimSpace(anno)
		if !strings.HasPrefix(trimmed, "@") {
			continue
		}
		trimmed = trimmed[1:] // strip '@'

		var key, valStr string
		idx := strings.IndexAny(trimmed, "=:")
		if idx != -1 {
			key = strings.TrimSpace(trimmed[:idx])
			valStr = strings.TrimSpace(trimmed[idx+1:])
		} else {
			key = strings.TrimSpace(trimmed)
			valStr = ""
		}

		if key == "" {
			continue
		}

		var parsedVal interface{}
		if valStr == "" {
			parsedVal = true
		} else {
			parsedVal = ParseAnnotationValue(valStr)
		}
		extensions[key] = parsedVal
	}

	return extensions, descs
}

// ParseAnnotationValue parses structured values inside annotations.
func ParseAnnotationValue(s string) interface{} {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return ""
	}

	if s[0] == '{' || s[0] == '[' || s[0] == '"' {
		tokens := scanAnnoTokens(s)
		parser := &annoParser{tokens: tokens}
		val, err := parser.parseValue()
		if err == nil {
			return val
		}
	}

	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	if isNumber(s) {
		if strings.Index(s, ".") == -1 {
			var val int
			if _, err := fmt.Sscanf(s, "%d", &val); err == nil {
				return val
			}
		} else {
			var val float64
			if _, err := fmt.Sscanf(s, "%f", &val); err == nil {
				return val
			}
		}
	}

	return s
}

type tokenType int

const (
	tokError tokenType = iota
	tokEOF
	tokEqual
	tokColon
	tokLBrace
	tokRBrace
	tokLBracket
	tokRBracket
	tokString
	tokNumber
	tokBool
	tokIdent
)

type annoToken struct {
	typ   tokenType
	value string
}

func scanAnnoTokens(s string) []annoToken {
	var tokens []annoToken
	runes := []rune(s)
	i := 0
	n := len(runes)

	for i < n {
		r := runes[i]
		if unicode.IsSpace(r) || r == ',' {
			i++
			continue
		}

		switch r {
		case '=':
			tokens = append(tokens, annoToken{typ: tokEqual, value: "="})
			i++
		case ':':
			tokens = append(tokens, annoToken{typ: tokColon, value: ":"})
			i++
		case '{':
			tokens = append(tokens, annoToken{typ: tokLBrace, value: "{"})
			i++
		case '}':
			tokens = append(tokens, annoToken{typ: tokRBrace, value: "}"})
			i++
		case '[':
			tokens = append(tokens, annoToken{typ: tokLBracket, value: "["})
			i++
		case ']':
			tokens = append(tokens, annoToken{typ: tokRBracket, value: "]"})
			i++
		case '"':
			start := i
			i++
			for i < n && runes[i] != '"' {
				if runes[i] == '\\' && i+1 < n {
					i += 2
				} else {
					i++
				}
			}
			if i < n {
				i++
			}
			tokens = append(tokens, annoToken{typ: tokString, value: string(runes[start:i])})
		default:
			start := i
			for i < n && !unicode.IsSpace(runes[i]) && runes[i] != ',' &&
				runes[i] != '=' && runes[i] != ':' &&
				runes[i] != '{' && runes[i] != '}' &&
				runes[i] != '[' && runes[i] != ']' {
				i++
			}
			val := string(runes[start:i])
			if val == "true" || val == "false" {
				tokens = append(tokens, annoToken{typ: tokBool, value: val})
			} else if isNumber(val) {
				tokens = append(tokens, annoToken{typ: tokNumber, value: val})
			} else {
				tokens = append(tokens, annoToken{typ: tokIdent, value: val})
			}
		}
	}

	return tokens
}

func isNumber(s string) bool {
	var dot bool
	for idx, r := range s {
		if r == '-' || r == '+' {
			if idx != 0 {
				return false
			}
		} else if r == '.' {
			if dot {
				return false
			}
			dot = true
		} else if !unicode.IsDigit(r) {
			return false
		}
	}
	return len(s) > 0 && s != "-" && s != "+" && s != "."
}

type annoParser struct {
	tokens []annoToken
	pos    int
}

func (p *annoParser) peek() annoToken {
	if p.pos >= len(p.tokens) {
		return annoToken{typ: tokEOF}
	}
	return p.tokens[p.pos]
}

func (p *annoParser) next() annoToken {
	t := p.peek()
	if t.typ != tokEOF {
		p.pos++
	}
	return t
}

func (p *annoParser) parseValue() (interface{}, error) {
	t := p.peek()
	switch t.typ {
	case tokString:
		p.next()
		if len(t.value) >= 2 && t.value[0] == '"' && t.value[len(t.value)-1] == '"' {
			val := t.value[1 : len(t.value)-1]
			val = strings.ReplaceAll(val, `\"`, `"`)
			val = strings.ReplaceAll(val, `\\`, `\`)
			val = strings.ReplaceAll(val, `\n`, "\n")
			val = strings.ReplaceAll(val, `\t`, "\t")
			return val, nil
		}
		return t.value, nil
	case tokNumber:
		p.next()
		var f float64
		if _, err := fmt.Sscanf(t.value, "%f", &f); err == nil {
			if strings.Index(t.value, ".") == -1 {
				var val int
				if _, err := fmt.Sscanf(t.value, "%d", &val); err == nil {
					return val, nil
				}
			}
			return f, nil
		}
		return t.value, nil
	case tokBool:
		p.next()
		return t.value == "true", nil
	case tokIdent:
		p.next()
		return t.value, nil
	case tokLBrace:
		p.next()
		obj := make(map[string]interface{})
		for {
			t = p.peek()
			if t.typ == tokRBrace || t.typ == tokEOF {
				break
			}
			if t.typ != tokIdent && t.typ != tokString {
				return nil, fmt.Errorf("expected object key, got %v", t.value)
			}
			keyToken := p.next()
			key := keyToken.value
			if keyToken.typ == tokString && len(key) >= 2 {
				key = key[1 : len(key)-1]
			}

			t = p.peek()
			if t.typ == tokEqual || t.typ == tokColon {
				p.next()
			}

			val, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			obj[key] = val
		}
		if p.peek().typ == tokRBrace {
			p.next()
		}
		return obj, nil
	case tokLBracket:
		p.next()
		var arr []interface{}
		for {
			t = p.peek()
			if t.typ == tokRBracket || t.typ == tokEOF {
				break
			}
			val, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			arr = append(arr, val)
		}
		if p.peek().typ == tokRBracket {
			p.next()
		}
		return arr, nil
	default:
		return nil, fmt.Errorf("unexpected token %v", t.value)
	}
}

// FormatAnnotationValue formats a value to its string representation.
func FormatAnnotationValue(val interface{}, indent string) string {
	switch v := val.(type) {
	case string:
		if strings.ContainsAny(v, " \t\n\r\"'{}[]:=,") || v == "true" || v == "false" || isNumber(v) || v == "" {
			return fmt.Sprintf("%q", v)
		}
		return v
	case bool:
		return fmt.Sprintf("%t", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int8:
		return fmt.Sprintf("%d", v)
	case int16:
		return fmt.Sprintf("%d", v)
	case int32:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case uint:
		return fmt.Sprintf("%d", v)
	case uint8:
		return fmt.Sprintf("%d", v)
	case uint16:
		return fmt.Sprintf("%d", v)
	case uint32:
		return fmt.Sprintf("%d", v)
	case uint64:
		return fmt.Sprintf("%d", v)
	case float32:
		return fmt.Sprintf("%g", v)
	case float64:
		return fmt.Sprintf("%g", v)
	case map[string]interface{}:
		var sb strings.Builder
		sb.WriteString("{\n")
		nextIndent := indent + "  "
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("%s%s = %s\n", nextIndent, k, FormatAnnotationValue(v[k], nextIndent)))
		}
		sb.WriteString(indent + "}")
		return sb.String()
	case []interface{}:
		var sb strings.Builder
		sb.WriteString("[\n")
		nextIndent := indent + "  "
		for _, item := range v {
			sb.WriteString(fmt.Sprintf("%s%s\n", nextIndent, FormatAnnotationValue(item, nextIndent)))
		}
		sb.WriteString(indent + "]")
		return sb.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// FormatAnnotation returns a formatted annotation string.
func FormatAnnotation(key string, val interface{}) string {
	return fmt.Sprintf("@%s = %s", key, FormatAnnotationValue(val, ""))
}

// SetSchemaQuality sets a standard schema quality on a JSONSchema from an annotation key and value.
// Returns true if the key was a known standard quality and was handled, false otherwise.
func (schema *JSONSchema) SetSchemaQuality(key string, val interface{}) bool {
	normKey := strings.ToLower(strings.ReplaceAll(key, "_", ""))
	switch normKey {
	case "title":
		if s, ok := val.(string); ok {
			schema.Title = s
		} else {
			schema.Title = fmt.Sprintf("%v", val)
		}
		return true
	case "description":
		if s, ok := val.(string); ok {
			schema.Description = s
		} else {
			schema.Description = fmt.Sprintf("%v", val)
		}
		return true
	case "default":
		schema.Default = val
		return true
	case "enum":
		if slice, ok := val.([]interface{}); ok {
			schema.Enum = slice
		} else if slice, ok := val.([]string); ok {
			var iSlice []interface{}
			for _, s := range slice {
				iSlice = append(iSlice, s)
			}
			schema.Enum = iSlice
		} else {
			schema.Enum = []interface{}{val}
		}
		return true
	case "min", "minimum":
		var f float64
		switch v := val.(type) {
		case int:
			f = float64(v)
		case float64:
			f = v
		default:
			if _, err := fmt.Sscanf(fmt.Sprintf("%v", val), "%f", &f); err != nil {
				return false
			}
		}
		schema.Minimum = &f
		return true
	case "max", "maximum":
		var f float64
		switch v := val.(type) {
		case int:
			f = float64(v)
		case float64:
			f = v
		default:
			if _, err := fmt.Sscanf(fmt.Sprintf("%v", val), "%f", &f); err != nil {
				return false
			}
		}
		schema.Maximum = &f
		return true
	case "exclusivemin", "exclusiveminimum":
		var f float64
		switch v := val.(type) {
		case int:
			f = float64(v)
		case float64:
			f = v
		default:
			if _, err := fmt.Sscanf(fmt.Sprintf("%v", val), "%f", &f); err != nil {
				return false
			}
		}
		schema.ExclusiveMinimum = &f
		return true
	case "exclusivemax", "exclusivemaximum":
		var f float64
		switch v := val.(type) {
		case int:
			f = float64(v)
		case float64:
			f = v
		default:
			if _, err := fmt.Sscanf(fmt.Sprintf("%v", val), "%f", &f); err != nil {
				return false
			}
		}
		schema.ExclusiveMaximum = &f
		return true
	case "minlength":
		var i int
		switch v := val.(type) {
		case int:
			i = v
		case float64:
			i = int(v)
		default:
			if _, err := fmt.Sscanf(fmt.Sprintf("%v", val), "%d", &i); err != nil {
				return false
			}
		}
		schema.MinLength = &i
		return true
	case "maxlength":
		var i int
		switch v := val.(type) {
		case int:
			i = v
		case float64:
			i = int(v)
		default:
			if _, err := fmt.Sscanf(fmt.Sprintf("%v", val), "%d", &i); err != nil {
				return false
			}
		}
		schema.MaxLength = &i
		return true
	case "pattern":
		if s, ok := val.(string); ok {
			schema.Pattern = s
		} else {
			schema.Pattern = fmt.Sprintf("%v", val)
		}
		return true
	case "format":
		if s, ok := val.(string); ok {
			schema.Format = s
		} else {
			schema.Format = fmt.Sprintf("%v", val)
		}
		return true
	case "minitems":
		var i int
		switch v := val.(type) {
		case int:
			i = v
		case float64:
			i = int(v)
		default:
			if _, err := fmt.Sscanf(fmt.Sprintf("%v", val), "%d", &i); err != nil {
				return false
			}
		}
		schema.MinItems = &i
		return true
	case "maxitems":
		var i int
		switch v := val.(type) {
		case int:
			i = v
		case float64:
			i = int(v)
		default:
			if _, err := fmt.Sscanf(fmt.Sprintf("%v", val), "%d", &i); err != nil {
				return false
			}
		}
		schema.MaxItems = &i
		return true
	case "uniqueitems":
		var b bool
		switch v := val.(type) {
		case bool:
			b = v
		default:
			s := strings.ToLower(fmt.Sprintf("%v", val))
			b = s == "true" || s == "1"
		}
		schema.UniqueItems = &b
		return true
	}
	return false
}

// GetSchemaQualities returns standard schema qualities as a map.
func (schema *JSONSchema) GetSchemaQualities() map[string]interface{} {
	m := make(map[string]interface{})
	if schema.Title != "" {
		m["title"] = schema.Title
	}
	if schema.Default != nil {
		m["default"] = schema.Default
	}
	if len(schema.Enum) > 0 {
		// Only include in qualities if we don't format it as a union type.
		if schema.Type != "string" && schema.Type != "" {
			m["enum"] = schema.Enum
		}
	}
	if schema.Minimum != nil {
		m["minimum"] = *schema.Minimum
	}
	if schema.Maximum != nil {
		m["maximum"] = *schema.Maximum
	}
	if schema.ExclusiveMinimum != nil {
		m["exclusiveMinimum"] = *schema.ExclusiveMinimum
	}
	if schema.ExclusiveMaximum != nil {
		m["exclusiveMaximum"] = *schema.ExclusiveMaximum
	}
	if schema.MinLength != nil {
		m["minLength"] = *schema.MinLength
	}
	if schema.MaxLength != nil {
		m["maxLength"] = *schema.MaxLength
	}
	if schema.Pattern != "" {
		m["pattern"] = schema.Pattern
	}
	if schema.Format != "" {
		m["format"] = schema.Format
	}
	if schema.MinItems != nil {
		m["minItems"] = *schema.MinItems
	}
	if schema.MaxItems != nil {
		m["maxItems"] = *schema.MaxItems
	}
	if schema.UniqueItems != nil {
		m["uniqueItems"] = *schema.UniqueItems
	}
	return m
}
