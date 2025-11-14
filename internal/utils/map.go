package utils

func DeepMerge(a, b map[string]any) map[string]any {
	out := make(map[string]any, len(a))
	for key, value := range a {
		out[key] = value
	}
	for key, value := range b {
		valueAsMap, ok := value.(map[string]any)
		if !ok {
			out[key] = value
			continue
		}
		outValue, ok := out[key]
		if !ok {
			out[key] = value
			continue
		}
		outValueAsMap, ok := outValue.(map[string]any)
		if !ok {
			out[key] = value
			continue
		}
		out[key] = DeepMerge(outValueAsMap, valueAsMap)
	}
	return out
}
