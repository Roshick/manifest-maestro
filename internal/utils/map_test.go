package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeepMerge(t *testing.T) {
	tests := []struct {
		name     string
		a        map[string]any
		b        map[string]any
		expected map[string]any
	}{
		{
			name:     "both empty",
			a:        map[string]any{},
			b:        map[string]any{},
			expected: map[string]any{},
		},
		{
			name:     "disjoint keys",
			a:        map[string]any{"a": 1},
			b:        map[string]any{"b": 2},
			expected: map[string]any{"a": 1, "b": 2},
		},
		{
			name:     "b overrides scalar in a",
			a:        map[string]any{"key": "old"},
			b:        map[string]any{"key": "new"},
			expected: map[string]any{"key": "new"},
		},
		{
			name: "nested maps are merged recursively",
			a: map[string]any{
				"outer": map[string]any{"keep": 1, "override": "old"},
			},
			b: map[string]any{
				"outer": map[string]any{"override": "new", "add": 2},
			},
			expected: map[string]any{
				"outer": map[string]any{"keep": 1, "override": "new", "add": 2},
			},
		},
		{
			name:     "map in b overrides scalar in a",
			a:        map[string]any{"key": "scalar"},
			b:        map[string]any{"key": map[string]any{"nested": true}},
			expected: map[string]any{"key": map[string]any{"nested": true}},
		},
		{
			name:     "map in b overrides non-map value in a",
			a:        map[string]any{"key": []string{"list"}},
			b:        map[string]any{"key": map[string]any{"nested": true}},
			expected: map[string]any{"key": map[string]any{"nested": true}},
		},
		{
			name:     "scalar in b overrides map in a",
			a:        map[string]any{"key": map[string]any{"nested": true}},
			b:        map[string]any{"key": "scalar"},
			expected: map[string]any{"key": "scalar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, DeepMerge(tt.a, tt.b))
		})
	}
}

func TestDeepMerge_DoesNotMutateInputs(t *testing.T) {
	a := map[string]any{"key": "a"}
	b := map[string]any{"key": "b"}

	result := DeepMerge(a, b)

	assert.Equal(t, map[string]any{"key": "b"}, result)
	assert.Equal(t, map[string]any{"key": "a"}, a)
	assert.Equal(t, map[string]any{"key": "b"}, b)
}
