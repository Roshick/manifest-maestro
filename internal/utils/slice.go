package utils

func Unique[V comparable](values []V) []V {
	indicators := IndicatorMap(values)
	unique := make([]V, 0)
	for value := range indicators {
		unique = append(unique, value)
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
