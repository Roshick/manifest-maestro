package utils

func Unique[V comparable](values []V) []V {
	seen := make(map[V]bool, len(values))
	unique := make([]V, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			unique = append(unique, value)
		}
	}
	return unique
}

func IndicatorMap[V comparable](values []V) map[V]bool {
	marker := make(map[V]bool)
	for _, value := range values {
		marker[value] = true
	}
	return marker
}
