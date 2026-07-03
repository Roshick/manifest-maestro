package helm

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChartBuildError_Unwrap(t *testing.T) {
	cause := errors.New("underlying build failure")
	err := NewChartBuildError(cause)

	assert.Contains(t, err.Error(), "failed to build Helm chart")
	assert.Contains(t, err.Error(), cause.Error())
	assert.ErrorIs(t, err, cause)
	assert.Equal(t, cause, errors.Unwrap(err))
}

func TestChartRenderError_Unwrap(t *testing.T) {
	cause := errors.New("underlying render failure")
	err := NewChartRenderError(cause)

	assert.Contains(t, err.Error(), "failed to render Helm chart")
	assert.Contains(t, err.Error(), cause.Error())
	assert.ErrorIs(t, err, cause)
	assert.Equal(t, cause, errors.Unwrap(err))
}

func TestChartReferenceInvalidError(t *testing.T) {
	err := NewChartReferenceInvalidError()
	assert.NotEmpty(t, err.Error())
}
