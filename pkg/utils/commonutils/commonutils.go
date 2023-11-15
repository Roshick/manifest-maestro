package commonutils

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

func Ptr[V any](value V) *V {
	return &value
}
