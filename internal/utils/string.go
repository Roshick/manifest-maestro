package utils

import (
	"strings"
)

func SplitUniqueNonEmpty(raw string, separator string) []string {
	values := strings.Split(raw, separator)
	k := 0
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
		if values[i] != "" {
			values[k] = values[i]
			k++
		}
	}
	return Unique(values[:k])
}
