package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnique(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		expected []string
	}{
		{
			name:     "no duplicates",
			values:   []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "removes duplicates preserving first occurrence order",
			values:   []string{"c", "a", "c", "b", "a"},
			expected: []string{"c", "a", "b"},
		},
		{
			name:     "empty slice",
			values:   []string{},
			expected: []string{},
		},
		{
			name:     "all identical",
			values:   []string{"x", "x", "x"},
			expected: []string{"x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Unique(tt.values))
		})
	}
}

func TestUnique_Ints(t *testing.T) {
	assert.Equal(t, []int{3, 1, 2}, Unique([]int{3, 1, 3, 2, 1}))
}

func TestIndicatorMap(t *testing.T) {
	assert.Equal(t, map[string]bool{"a": true, "b": true}, IndicatorMap([]string{"a", "b", "a"}))
	assert.Equal(t, map[int]bool{}, IndicatorMap([]int{}))
}
