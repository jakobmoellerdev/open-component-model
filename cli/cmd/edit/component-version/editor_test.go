package componentversion

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveEditor_FlagTakesPrecedence(t *testing.T) {
	result := resolveEditor("nano")
	assert.Equal(t, "nano", result)
}

func TestResolveEditor_VisualEnv(t *testing.T) {
	t.Setenv("VISUAL", "code --wait")
	t.Setenv("EDITOR", "nano")

	result := resolveEditor("")
	assert.Equal(t, "code --wait", result)
}

func TestResolveEditor_EditorEnv(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "nano")

	result := resolveEditor("")
	assert.Equal(t, "nano", result)
}

func TestResolveEditor_DefaultVi(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	result := resolveEditor("")
	assert.Equal(t, "vi", result)
}
