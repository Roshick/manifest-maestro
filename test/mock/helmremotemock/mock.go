package helmremotemock

import (
	"context"
	"net/url"
	"sync"
	"sync/atomic"
)

// ChartMock implements cache.HelmChartRemote.
type ChartMock struct {
	GetChartCallCount atomic.Int32

	mu     sync.RWMutex
	charts map[string][]byte
}

func NewChartMock() *ChartMock {
	return &ChartMock{
		charts: make(map[string][]byte),
	}
}

func (m *ChartMock) AddChart(chartURL string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.charts[chartURL] = data
}

func (m *ChartMock) GetChart(_ context.Context, chartURL url.URL) ([]byte, error) {
	m.GetChartCallCount.Add(1)
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.charts[chartURL.String()]
	if !ok {
		return nil, &ChartNotFoundError{URL: chartURL.String()}
	}
	return data, nil
}

type ChartNotFoundError struct {
	URL string
}

func (e *ChartNotFoundError) Error() string {
	return "chart not found: " + e.URL
}

// IndexMock implements cache.HelmIndexRemote.
type IndexMock struct {
	GetIndexCallCount atomic.Int32

	mu      sync.RWMutex
	indexes map[string][]byte
}

func NewIndexMock() *IndexMock {
	return &IndexMock{
		indexes: make(map[string][]byte),
	}
}

func (m *IndexMock) AddIndex(repoURL string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.indexes[repoURL] = data
}

func (m *IndexMock) GetIndex(_ context.Context, repoURL url.URL) ([]byte, error) {
	m.GetIndexCallCount.Add(1)
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.indexes[repoURL.String()]
	if !ok {
		return nil, &IndexNotFoundError{URL: repoURL.String()}
	}
	return data, nil
}

type IndexNotFoundError struct {
	URL string
}

func (e *IndexNotFoundError) Error() string {
	return "index not found: " + e.URL
}

