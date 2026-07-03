package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitUniqueNonEmpty(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		separator string
		expected  []string
	}{
		{
			name:      "simple split",
			raw:       "a,b,c",
			separator: ",",
			expected:  []string{"a", "b", "c"},
		},
		{
			name:      "trims whitespace",
			raw:       " a , b , c ",
			separator: ",",
			expected:  []string{"a", "b", "c"},
		},
		{
			name:      "drops empty values",
			raw:       "a,,b,   ,c,",
			separator: ",",
			expected:  []string{"a", "b", "c"},
		},
		{
			name:      "removes duplicates preserving order",
			raw:       "b,a,b,c,a",
			separator: ",",
			expected:  []string{"b", "a", "c"},
		},
		{
			name:      "duplicates after trimming",
			raw:       "a, a ,a",
			separator: ",",
			expected:  []string{"a"},
		},
		{
			name:      "empty input",
			raw:       "",
			separator: ",",
			expected:  []string{},
		},
		{
			name:      "only separators and whitespace",
			raw:       " , ,,  ,",
			separator: ",",
			expected:  []string{},
		},
		{
			name:      "different separator",
			raw:       "a;b;c",
			separator: ";",
			expected:  []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, SplitUniqueNonEmpty(tt.raw, tt.separator))
		})
	}
}
