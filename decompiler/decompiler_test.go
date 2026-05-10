package decompiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/theirish/fml/compiler"
	"github.com/theirish/fml/parser"
	"gopkg.in/yaml.v3"
)

func TestDecompiler_NamespacedTargets(t *testing.T) {
	input := `session("s") {
    call("tool") -> context:var1 { }
    schema {
        result: string
    }
}
`
	p, _ := parser.NewParser()
	plan, _ := p.ParseString("test.frags", input)
	comp := compiler.New(plan)
	planYAML, _ := comp.Compile()

	dec := New(planYAML)
	output, err := dec.Decompile()
	assert.NoError(t, err)

	assert.Contains(t, output, `call("tool") -> context:var1`)

	// Round-trip
	plan2, err := p.ParseString("roundtrip.frags", output)
	assert.NoError(t, err)
	comp2 := compiler.New(plan2)
	planYAML2, err := comp2.Compile()

	y1, _ := yaml.Marshal(planYAML)
	y2, _ := yaml.Marshal(planYAML2)
	assert.Equal(t, string(y1), string(y2))
}
