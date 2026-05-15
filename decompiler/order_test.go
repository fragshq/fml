package decompiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/theirish81/fml/compiler"
	"github.com/theirish81/fml/parser"
	"gopkg.in/yaml.v3"
)

func TestSessionOrderingRoundTrip(t *testing.T) {
	// FML with sessions in specific, non-alphabetical order
	fml := `session("s3") {
    - "p3"
}

session("s2", target="s2_output") {
    - "p2"
    schema {
        b: string
    }
}

session("s1") {
    - "p1"
    schema {
        a: string
    }
}
`
	// 1. Parse
	p, err := parser.NewParser()
	assert.NoError(t, err)
	ast, err := p.ParseString("test.fml", fml)
	assert.NoError(t, err)

	// 2. Compile
	c := compiler.New(ast)
	plan, err := c.Compile()
	assert.NoError(t, err)

	// 3. Marshal to YAML (simulates storage)
	outYAML, err := yaml.Marshal(plan)
	assert.NoError(t, err)

	// 4. Unmarshal back to PlanYAML (simulates loading)
	var planYAML compiler.PlanYAML
	err = yaml.Unmarshal(outYAML, &planYAML)
	assert.NoError(t, err)

	// 5. Decompile
	dec := New(&planYAML)
	decompiled, err := dec.Decompile()
	assert.NoError(t, err)

	// 6. Verify original order is preserved
	// We expect the exact same order: s3, s2, s1
	assert.Equal(t, fml, decompiled)
}

func TestSessionRedefinitionOrdering(t *testing.T) {
	// Testing that re-defining a session doesn't move its position
	fml := `session("s2") {
    - "p2"
}

session("s1") {
    - "p1"
}

session("s2") {
    - "p2 updated"
}
`
	// The compiler merges these, usually keeping the first occurrence's position
	p, _ := parser.NewParser()
	ast, _ := p.ParseString("test.fml", fml)
	c := compiler.New(ast)
	plan, _ := c.Compile()

	outYAML, _ := yaml.Marshal(plan)
	var planYAML compiler.PlanYAML
	_ = yaml.Unmarshal(outYAML, &planYAML)

	dec := New(&planYAML)
	decompiled, _ := dec.Decompile()

	expected := `session("s2") {
    - "p2 updated"
}

session("s1") {
    - "p1"
}
`
	assert.Equal(t, expected, decompiled)
}
