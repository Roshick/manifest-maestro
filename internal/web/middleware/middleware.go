package middleware

import (
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/Roshick/go-autumn-slog/pkg/logging"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"github.com/StephanHCB/go-backend-service-common/web/middleware/requestid"
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
)

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

func AddRequestIDToContextLogger(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx)
		if logger != nil {
			requestID := requestid.GetReqID(ctx)
			if requestID != "" {
				logger = logger.With(LogFieldRequestID, requestID)
			}
		}
		ctx = logging.ContextWithLogger(ctx, logger)

		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

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
			ctx := r.Context()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			t1 := time.Now()
			defer func() {
				requestInfo := fmt.Sprintf("%s %s %d", r.Method, r.URL.Path, ww.Status())
				for _, re := range excludeRegexes {
					if re.MatchString(requestInfo) {
						return
					}
				}

				logger := logging.FromContext(ctx)
				if logger != nil {
					logger = logger.With(
						LogFieldRequestMethod, r.Method,
						LogFieldResponseStatus, ww.Status(),
						LogFieldURLPath, r.URL.Path,
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

			next.ServeHTTP(ww, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}
}
