package utils

func DefaultIfNil[V comparable](value *V, defaultValue V) V {
	if value == nil {
		return defaultValue
	}
	return *value
}

func DefaultIfEmpty[V comparable](value *V, defaultValue V) V {
	if value == nil || *value == *new(V) {
		return defaultValue
	}
	return *value
}

func IsEmpty[V comparable](value *V) bool {
	return value == nil || *value == *new(V)
}

func Ptr[V any](value V) *V {
	return &value
}
