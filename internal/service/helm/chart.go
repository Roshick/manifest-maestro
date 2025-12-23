package helm

import (
	"fmt"

	openapi "github.com/Roshick/manifest-maestro-api"
	"github.com/Roshick/manifest-maestro/internal/utils"
	"github.com/Roshick/manifest-maestro/pkg/filesystem"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/strvals"
	"sigs.k8s.io/yaml"
)

type Chart struct {
	chart      *chart.Chart
	fileSystem *filesystem.FileSystem
	targetPath string
}

func (c *Chart) DefaultValues() map[string]any {
	return c.chart.Values
}

func (c *Chart) MergeValues(parameters openapi.HelmRenderParameters) (map[string]any, error) {
	values := utils.DeepMerge(parameters.ComplexValues, make(map[string]any))

	for _, fileName := range parameters.ValueFiles {
		filePath := c.fileSystem.Join(c.targetPath, fileName)
		if c.fileSystem.Exists(filePath) {
			valueFile, err := c.fileSystem.ReadFile(filePath)
			if err != nil {
				return nil, err
			}
			tmpValues := make(map[string]any)
			if err = yaml.Unmarshal(valueFile, &tmpValues); err != nil {
				return nil, err
			}
			values = utils.DeepMerge(tmpValues, values)
		} else if parameters.IgnoreMissingValueFiles == nil || !*parameters.IgnoreMissingValueFiles {
			return nil, fmt.Errorf("repository is missing value file at '%s'", filePath)
		}
	}

	for _, value := range append(c.flattenValues(parameters.Values), parameters.ValuesFlat...) {
		if err := strvals.ParseInto(value, values); err != nil {
			return nil, err
		}
	}

	for _, value := range append(c.flattenValues(parameters.StringValues), parameters.StringValuesFlat...) {
		if err := strvals.ParseIntoString(value, values); err != nil {
			return nil, err
		}
	}

	return values, nil
}

func (c *Chart) flattenValues(values *map[string]string) []string {
	if values == nil {
		return nil
	}

	flattenedValues := make([]string, 0)
	for key, value := range *values {
		flattenedValues = append(flattenedValues, fmt.Sprintf("%s=%s", key, value))
	}
	return flattenedValues
}
