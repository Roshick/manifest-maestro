package stringutils

import (
	"strings"

	"github.com/Roshick/manifest-maestro/pkg/utils/sliceutils"
)

func SplitUniqueNonEmpty(raw string, separator string) []string {
	values := strings.Split(raw, separator)
	k := 0
	for i, n := range values {
		values[i] = strings.TrimSpace(values[i])
		if values[i] != "" {
			values[k] = n
			k++
		}
	}
	return sliceutils.Unique(values[:k])
}
