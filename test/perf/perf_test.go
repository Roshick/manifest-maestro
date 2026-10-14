//go:build perf

// Package perf replays recorded API requests against the V1 handler stack (caches, providers, renderers) with
// in-memory git and helm remotes, verifies the responses against golden files and reports time and memory usage.
//
// Scenarios live in test/resources/perf/<name> (git-ignored, as they usually contain non-public data):
//
//	scenario.json      {"repositories": {"<url>": {"<ref>": "<commit>"}}, "charts": {"<chart url>": "<file>"}}
//	repos/<commit>/    snapshot of the repository at that commit, e.g. via `git archive <commit> | tar -x`
//	charts/<file>      chart archives served by the helm remote
//	requests/<n>.json  request bodies; the name is "<anything>-<endpoint>" with "/" in the endpoint replaced by
//	                   "_", e.g. "1-helm_actions_render-chart.json"
//	golden/<n>.json    expected response bodies, written with PERF_UPDATE_GOLDEN=1
//
// Usage: go test -tags perf ./test/perf/ -run TestPerf -v -count=1 [-cpuprofile ...] [-memprofile ...]
// Optional: PERF_RUNS (warm runs per request, default 5), PERF_SCENARIOS (comma separated scenario names).
package perf_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	aucache "github.com/Roshick/go-autumn-synchronisation/pkg/cache"
	"github.com/Roshick/manifest-maestro/internal/service/cache"
	"github.com/Roshick/manifest-maestro/internal/service/helm"
	"github.com/Roshick/manifest-maestro/internal/service/kustomize"
	"github.com/Roshick/manifest-maestro/internal/web/controller"
	"github.com/Roshick/manifest-maestro/test/mock/gitmock"
	"github.com/go-chi/chi/v5"
	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/require"
)

const dataDir = "../resources/perf"

type scenarioConfig struct {
	Repositories map[string]map[string]string `json:"repositories"`
	Charts       map[string]string            `json:"charts"`
}

type request struct {
	name string
	path string
	body []byte
}

type helmRemote struct {
	charts map[string][]byte
}

func (h *helmRemote) GetChart(_ context.Context, chartURL url.URL) ([]byte, error) {
	if data, ok := h.charts[chartURL.String()]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("unknown chart %s", chartURL.String())
}

func (h *helmRemote) GetIndex(_ context.Context, repositoryURL url.URL) ([]byte, error) {
	return nil, fmt.Errorf("unknown index %s", repositoryURL.String())
}

func loadScenario(t *testing.T, dir string) (http.Handler, []request) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "scenario.json"))
	require.NoError(t, err)
	var cfg scenarioConfig
	require.NoError(t, json.Unmarshal(raw, &cfg))

	gitMock := gitmock.NewMock().
		WithToHash(func(_ context.Context, repositoryURL string, ref string) (string, error) {
			if commit, ok := cfg.Repositories[repositoryURL][ref]; ok {
				return commit, nil
			}
			return "", fmt.Errorf("unknown reference %s %s", repositoryURL, ref)
		}).
		WithCloneCommit(func(_ context.Context, _ string, commit string) (*git.Repository, error) {
			return gitmock.CreateRepoFromDir(filepath.Join(dir, "repos", commit))
		})

	remote := &helmRemote{charts: make(map[string][]byte)}
	for chartURL, file := range cfg.Charts {
		data, readErr := os.ReadFile(filepath.Join(dir, "charts", file))
		require.NoError(t, readErr)
		remote.charts[chartURL] = data
	}

	// same composition as internal/wiring with the default (in-memory) caches
	gitRepositoryCache := cache.NewGitRepositoryCache(gitMock, aucache.NewMemoryCache[[]byte]())
	helmIndexCache := cache.NewHelmIndexCache(remote, aucache.NewMemoryCache[[]byte]())
	helmChartCache := cache.NewHelmChartCache(remote, helmIndexCache, aucache.NewMemoryCache[[]byte]())
	router := chi.NewRouter()
	controller.NewV1Controller(nil, nil,
		helm.NewChartProvider(helmChartCache, gitRepositoryCache), helm.NewChartRenderer(nil),
		kustomize.NewKustomizationProvider(gitRepositoryCache), kustomize.NewKustomizationRenderer(),
	).WireUp(context.Background(), router)

	files, err := filepath.Glob(filepath.Join(dir, "requests", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	requests := make([]request, 0, len(files))
	for _, file := range files {
		body, readErr := os.ReadFile(file)
		require.NoError(t, readErr)
		name := strings.TrimSuffix(filepath.Base(file), ".json")
		_, endpoint, found := strings.Cut(name, "-")
		require.Truef(t, found, "request file name %q lacks an endpoint", name)
		requests = append(requests, request{
			name: name,
			path: "/rest/api/v1/" + strings.ReplaceAll(endpoint, "_", "/"),
			body: body,
		})
	}
	return router, requests
}

func serve(handler http.Handler, req request) (int, []byte) {
	httpReq := httptest.NewRequest(http.MethodPost, req.path, bytes.NewReader(req.body))
	httpReq.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httpReq)
	return rec.Code, rec.Body.Bytes()
}

type measurement struct {
	wall       time.Duration
	totalAlloc uint64
	mallocs    uint64
	peakHeap   uint64
}

// measure reports the resources used by fn. peakHeap is the sampled maximum of the live heap above the heap before
// fn started, i.e. an approximation of the additional memory needed.
func measure(fn func()) measurement {
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	var peak atomic.Uint64
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		var stats runtime.MemStats
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				runtime.ReadMemStats(&stats)
				if stats.HeapAlloc > peak.Load() {
					peak.Store(stats.HeapAlloc)
				}
			}
		}
	})

	start := time.Now()
	fn()
	wall := time.Since(start)
	close(done)
	wg.Wait()

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	peakHeap := max(peak.Load(), after.HeapAlloc)
	return measurement{
		wall:       wall,
		totalAlloc: after.TotalAlloc - before.TotalAlloc,
		mallocs:    after.Mallocs - before.Mallocs,
		peakHeap:   peakHeap - min(peakHeap, before.HeapAlloc),
	}
}

func median(ms []measurement) measurement {
	pick := func(value func(m measurement) uint64) uint64 {
		values := make([]uint64, 0, len(ms))
		for _, m := range ms {
			values = append(values, value(m))
		}
		slices.Sort(values)
		return values[len(values)/2]
	}
	return measurement{
		wall:       time.Duration(pick(func(m measurement) uint64 { return uint64(m.wall) })),
		totalAlloc: pick(func(m measurement) uint64 { return m.totalAlloc }),
		mallocs:    pick(func(m measurement) uint64 { return m.mallocs }),
		peakHeap:   pick(func(m measurement) uint64 { return m.peakHeap }),
	}
}

func row(scenario string, name string, cold string, m measurement) string {
	mib := func(b uint64) string { return fmt.Sprintf("%.1f MiB", float64(b)/(1<<20)) }
	return fmt.Sprintf("| %s | %s | %s | %v | %s | %d | %s |", scenario, name, cold,
		m.wall.Round(100*time.Microsecond), mib(m.totalAlloc), m.mallocs, mib(m.peakHeap))
}

func runs() int {
	if n, err := strconv.Atoi(os.Getenv("PERF_RUNS")); err == nil && n > 0 {
		return n
	}
	return 5
}

func scenarios(t *testing.T) []string {
	if raw := os.Getenv("PERF_SCENARIOS"); raw != "" {
		names := strings.Split(raw, ",")
		for _, name := range names {
			require.Truef(t, filepath.IsLocal(name), "invalid scenario name %q", name)
		}
		return names
	}
	entries, err := os.ReadDir(dataDir)
	if os.IsNotExist(err) {
		t.Skipf("no scenarios in %s", dataDir)
	}
	require.NoError(t, err)
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names
}

func checkGolden(t *testing.T, goldenFile string, name string, body []byte) {
	t.Helper()
	if os.Getenv("PERF_UPDATE_GOLDEN") != "" {
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenFile), 0o755))
		require.NoError(t, os.WriteFile(goldenFile, body, 0o600))
		return
	}
	expected, err := os.ReadFile(goldenFile)
	require.NoError(t, err)
	if !bytes.Equal(expected, body) {
		require.JSONEqf(t, string(expected), string(body), "%s differs from golden file", name)
		t.Errorf("%s is equivalent to but not identical with golden file", name)
	}
}

// measureConcurrently sends all requests of a scenario at once, as a client rendering several apps would.
func measureConcurrently(t *testing.T, handler http.Handler, requests []request) measurement {
	t.Helper()
	var failed atomic.Bool
	m := measure(func() {
		var wg sync.WaitGroup
		for _, req := range requests {
			wg.Go(func() {
				if status, _ := serve(handler, req); status != http.StatusOK {
					failed.Store(true)
				}
			})
		}
		wg.Wait()
	})
	require.False(t, failed.Load(), "concurrent request failed")
	return m
}

func TestPerf(t *testing.T) {
	var report []string
	for _, scenario := range scenarios(t) {
		t.Run(scenario, func(t *testing.T) {
			dir := filepath.Join(dataDir, scenario)
			handler, requests := loadScenario(t, dir)

			// the first call of each request fills the caches (cold), all further calls hit them (warm)
			for _, req := range requests {
				var status int
				var body []byte
				cold := measure(func() { status, body = serve(handler, req) })
				require.Equalf(t, http.StatusOK, status, "%s: %s", req.name, body)

				checkGolden(t, filepath.Join(dir, "golden", req.name+".json"), req.name, body)

				warm := make([]measurement, 0, runs())
				for range runs() {
					warm = append(warm, measure(func() { status, body = serve(handler, req) }))
					require.Equal(t, http.StatusOK, status)
				}
				report = append(
					report,
					row(scenario, req.name, cold.wall.Round(time.Millisecond).String(), median(warm)),
				)
			}

			concurrent := make([]measurement, 0, runs())
			for range runs() {
				concurrent = append(concurrent, measureConcurrently(t, handler, requests))
			}
			report = append(report, row(scenario, "all concurrently", "", median(concurrent)))
		})
	}

	t.Logf("\n| scenario | request | cold | warm (median of %d) | alloc | mallocs | peak heap |\n"+
		"|---|---|---|---|---|---|---|\n%s", runs(), strings.Join(report, "\n"))
}
