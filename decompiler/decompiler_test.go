package decompiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/theirish/fml/compiler"
	"github.com/theirish/fml/parser"
	"gopkg.in/yaml.v3"
)

func TestDecompiler_Basic(t *testing.T) {
	input := `system("Sys")
parameters {
    # The count
    limit: int = 10 # inline
}
set global = "hello"
session("s1", target="res", after="prev", expect=1==1, iterate=context.items) {
    use search
    use mcp kb
    call("tool") -> x {
        arg1 = "val"
        code( args.arg1.toUpper() )
    }
    context "ctx"
    - First line
      second line
    schema {
        field: string
    }
}
`
	// 1. Compile to YAML
	p, _ := parser.NewParser()
	plan, err := p.ParseString("test.frags", input)
	assert.NoError(t, err)
	comp := compiler.New(plan)
	planYAML, err := comp.Compile()
	assert.NoError(t, err)

	// 2. Decompile back to FML
	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	// 3. Verify key constructs exist in decompiled output
	assert.Contains(t, output, `system("Sys")`)
	assert.Contains(t, output, `limit: int = 10`)
	assert.Contains(t, output, `set global = "hello"`)
	assert.Contains(t, output, `session("s1", target="res", after="prev", expect=1==1, iterate=context.items)`)
	assert.Contains(t, output, `use search`)
	assert.Contains(t, output, `use mcp kb`)
	assert.Contains(t, output, `call("tool") -> x`)
	assert.Contains(t, output, `- First line`)
	assert.Contains(t, output, `field: string`)

	// 4. Round-trip: Compile the decompiled output and compare with original YAML
	plan2, err := p.ParseString("roundtrip.frags", output)
	if err != nil {
		t.Fatalf("Failed to parse decompiled output: %v\nOutput was:\n%s", err, output)
	}
	comp2 := compiler.New(plan2)
	planYAML2, err := comp2.Compile()
	assert.NoError(t, err)

	// Compare YAML strings (ignoring comments for simplicity in comparison)
	y1, _ := yaml.Marshal(planYAML)
	y2, _ := yaml.Marshal(planYAML2)
	assert.Equal(t, string(y1), string(y2))
}
