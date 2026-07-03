package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultIfNil(t *testing.T) {
	assert.Equal(t, "default", DefaultIfNil(nil, "default"))
	assert.Equal(t, "value", DefaultIfNil(Ptr("value"), "default"))
	// zero value is kept, only nil falls back to the default
	assert.Equal(t, "", DefaultIfNil(Ptr(""), "default"))
	assert.Equal(t, 0, DefaultIfNil(Ptr(0), 42))
}

func TestDefaultIfEmpty(t *testing.T) {
	assert.Equal(t, "default", DefaultIfEmpty(nil, "default"))
	assert.Equal(t, "default", DefaultIfEmpty(Ptr(""), "default"))
	assert.Equal(t, "value", DefaultIfEmpty(Ptr("value"), "default"))
	assert.Equal(t, 42, DefaultIfEmpty(Ptr(0), 42))
	assert.Equal(t, 7, DefaultIfEmpty(Ptr(7), 42))
}

func TestIsEmpty(t *testing.T) {
	assert.True(t, IsEmpty[string](nil))
	assert.True(t, IsEmpty(Ptr("")))
	assert.False(t, IsEmpty(Ptr("value")))
	assert.True(t, IsEmpty(Ptr(0)))
	assert.False(t, IsEmpty(Ptr(1)))
}

func TestPtr(t *testing.T) {
	value := Ptr("hello")
	require.NotNil(t, value)
	assert.Equal(t, "hello", *value)

	number := Ptr(42)
	require.NotNil(t, number)
	assert.Equal(t, 42, *number)
}
