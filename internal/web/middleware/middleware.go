package middleware

import (
	"context"
	"fmt"
	"github.com/Roshick/manifest-maestro/pkg/utils/commonutils"
	"github.com/go-chi/chi/v5"
	"github.com/go-http-utils/headers"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/Roshick/go-autumn-slog/pkg/logging"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	LogFieldRequestMethod  = "request-method"
	LogFieldRequestID      = "request-id"
	LogFieldResponseStatus = "response-status"
	LogFieldURLPath        = "url-path"
	LogFieldUserAgent      = "user-agent"
	LogFieldEventDuration  = "event-duration"
	LogFieldLogger         = "logger"
	LogFieldStackTrace     = "stack-trace"
)

// AddLoggerToContext //

func AddLoggerToContext(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if slogging, ok := aulogging.Logger.(*logging.Logging); ok {
			ctx = logging.ContextWithLogger(ctx, slogging.Logger())
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

// AddRequestIDToContext //

type AddRequestIDToContextOptions struct {
	RequestIDHeader string
	RequestIDFunc   func() string
}

type requestIDContextKey struct{}

func RequestIDFromContext(ctx context.Context) *string {
	if value := ctx.Value(requestIDContextKey{}); value != nil {
		return commonutils.Ptr(value.(string))
	}
	return nil
}

func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

func CreateAddRequestIDToContext(options AddRequestIDToContextOptions) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if requestID := r.Header.Get(options.RequestIDHeader); requestID == "" {
				requestID = options.RequestIDFunc()
				ctx = ContextWithRequestID(ctx, requestID)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}
}

// AddRequestIDToContextLogger //

func AddRequestIDToContextLogger(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if logger := logging.FromContext(ctx); logger != nil {
			requestID := RequestIDFromContext(ctx)
			if commonutils.DefaultIfEmpty(requestID, "") != "" {
				logger = logger.With(LogFieldRequestID, *requestID)
			}
			ctx = logging.ContextWithLogger(ctx, logger)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

// AddRequestIDToResponseHeader //

type AddRequestIDToResponseHeaderOptions struct {
	RequestIDHeader string
}

func CreateAddRequestIDToResponseHeader(options AddRequestIDToResponseHeaderOptions) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if requestID := RequestIDFromContext(ctx); commonutils.DefaultIfEmpty(requestID, "") != "" {
				w.Header().Set(options.RequestIDHeader, *requestID)
			}

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

// AddRequestResponseContextLogging //

type AddRequestResponseContextLoggingOptions struct {
	ExcludeLogging []string
}

func CreateAddRequestResponseContextLogging(options AddRequestResponseContextLoggingOptions) func(next http.Handler) http.Handler {
	excludeRegexes := make([]*regexp.Regexp, 0)
	for _, pattern := range options.ExcludeLogging {
		fullMatchPattern := "^" + pattern + "$"
		re, err := regexp.Compile(fullMatchPattern)
		if err != nil {
			aulogging.Logger.NoCtx().Error().WithErr(err).Printf("failed to compile exclude logging pattern '%s', skipping pattern", fullMatchPattern)
		} else {
			excludeRegexes = append(excludeRegexes, re)
		}
	}

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			t1 := time.Now()
			defer func() {
				ctx := r.Context()

				requestInfo := fmt.Sprintf("%s %s %d", r.Method, r.URL.Path, ww.Status())
				for _, re := range excludeRegexes {
					if re.MatchString(requestInfo) {
						return
					}
				}

				if logger := logging.FromContext(ctx); logger != nil {
					logger = logger.With(
						LogFieldRequestMethod, r.Method,
						LogFieldResponseStatus, ww.Status(),
						LogFieldURLPath, r.URL.RawPath,
						LogFieldUserAgent, r.UserAgent(),
						LogFieldLogger, "request.incoming",
						LogFieldEventDuration, time.Since(t1).Microseconds(),
					)
					subCtx := logging.ContextWithLogger(ctx, logger)
					if ww.Status() >= http.StatusInternalServerError {
						aulogging.Logger.Ctx(subCtx).Error().Print("request")
					} else {
						aulogging.Logger.Ctx(subCtx).Info().Print("request")
					}
				}
			}()

			next.ServeHTTP(ww, r)
		}
		return http.HandlerFunc(fn)
	}
}

// AddRequestTimeout //

type AddRequestTimeoutOptions struct {
	RequestTimeoutInSeconds int
}

func CreateAddRequestTimeout(options AddRequestTimeoutOptions) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			ctx, cancel := context.WithTimeout(ctx, time.Duration(options.RequestTimeoutInSeconds)*time.Second)
			defer cancel()

			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}
}

// AddContextCancelLogging //

type LogContextCancellationOptions struct {
	Description string
}

func CreateLogContextCancellation(options LogContextCancellationOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			next.ServeHTTP(w, r)

			cause := context.Cause(ctx)
			if cause != nil {
				aulogging.Logger.NoCtx().Info().WithErr(cause).Printf("context '%s' is cancelled", options.Description)
			}
		}
		return http.HandlerFunc(fn)
	}
}

// RecoverPanic //

func RecoverPanic(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			ctx := r.Context()
			rvr := recover()
			if rvr != nil && rvr != http.ErrAbortHandler {
				if logger := logging.FromContext(ctx); logger != nil {
					subCtx := logging.ContextWithLogger(ctx, logger.With(LogFieldStackTrace, debug.Stack()))
					aulogging.Logger.Ctx(subCtx).Error().Print("recovered from panic")
				}
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

// HandleCORS //

type HandleCORSOptions struct {
	AllowOrigin             string
	AdditionalAllowHeaders  []string
	AdditionalExposeHeaders []string
}

func CreateHandleCORS(options HandleCORSOptions) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(headers.AccessControlAllowOrigin, options.AllowOrigin)

			w.Header().Set(headers.AccessControlAllowMethods, strings.Join([]string{
				http.MethodGet,
				http.MethodHead,
				http.MethodPost,
				http.MethodPut,
				http.MethodPatch,
				http.MethodDelete,
			}, ", "))

			w.Header().Set(headers.AccessControlAllowHeaders, strings.Join(append([]string{
				headers.Accept,
				headers.ContentType,
			}, options.AdditionalAllowHeaders...), ", "))

			w.Header().Set(headers.AccessControlAllowCredentials, "true")

			w.Header().Set(headers.AccessControlExposeHeaders, strings.Join(append([]string{
				headers.CacheControl,
				headers.ContentSecurityPolicy,
				headers.ContentType,
				headers.Location,
			}, options.AdditionalExposeHeaders...), ", "))

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

// RecordRequestMetrics //

func CreateRecordRequestMetrics() func(next http.Handler) http.Handler {
	requests := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "http_server_requests_seconds",
			Help: "How long it took to process requests, partitioned by status code, method and HTTP path (grouped by patterns).",
		},
		[]string{"method", "status", "uri"},
	)
	prometheus.MustRegister(requests)

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			routeCtx := chi.RouteContext(r.Context())
			routePattern := strings.Join(routeCtx.RoutePatterns, "")
			routePattern = strings.Replace(routePattern, "/*/", "/", -1)

			requests.WithLabelValues(r.Method, strconv.Itoa(ww.Status()), routePattern).Observe(float64(time.Since(start).Microseconds()) / 1000000)
		}
		return http.HandlerFunc(fn)
	}
}
