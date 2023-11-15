package helper

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	apimodel "github.com/Roshick/manifest-maestro/api"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"github.com/StephanHCB/go-backend-service-common/web/util/media"
	"github.com/go-http-utils/headers"
	"gopkg.in/yaml.v3"
)

func BooleanQueryParam(r *http.Request, key string, defaultValue bool) (bool, error) {
	query := r.URL.Query()
	value := query.Get(key)
	if value == "" {
		return defaultValue, nil
	}
	return strconv.ParseBool(query.Get(key))
}

// --- success ---

const ContentTypeApplicationYaml = "application/x-yaml"

func Success(ctx context.Context, w http.ResponseWriter, r *http.Request, response any) {
	acceptEncodingHeader := r.Header.Get(headers.Accept)
	if strings.Contains(acceptEncodingHeader, ContentTypeApplicationYaml) {
		w.Header().Set(headers.ContentType, ContentTypeApplicationYaml)
		WriteYAML(ctx, w, response)
	} else {
		w.Header().Set(headers.ContentType, media.ContentTypeApplicationJson)
		WriteJSON(ctx, w, response)
	}
}

func NoContent(_ context.Context, w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// --- error handlers ---

func HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error, timeStamp time.Time) {
	UnexpectedErrorHandler(ctx, w, r, err.Error(), timeStamp)
}

func NotFoundErrorHandler(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	logMessage string,
	timeStamp time.Time,
) {
	aulogging.Logger.Ctx(ctx).Info().Printf("not found: %s", logMessage)
	errorHandler(ctx, w, r, "notfound", http.StatusNotFound, logMessage, timeStamp)
}

func BadRequestErrorHandler(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	logMessage string,
	timeStamp time.Time,
) {
	aulogging.Logger.Ctx(ctx).Info().Printf("bad request: %s", logMessage)
	errorHandler(ctx, w, r, "input.invalid", http.StatusBadRequest, logMessage, timeStamp)
}

func ConflictErrorhandler(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	logMessage string,
	timeStamp time.Time,
) {
	aulogging.Logger.Ctx(ctx).Info().Printf("conflict: %s", logMessage)
	errorHandler(ctx, w, r, "conflict", http.StatusConflict, logMessage, timeStamp)
}

func UnprocessableEntityErrorHandler(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	logMessage string,
	timeStamp time.Time,
) {
	aulogging.Logger.Ctx(ctx).Info().Printf("unprocessable entity: %s", logMessage)
	errorHandler(ctx, w, r, "deployment-repository.invalid", http.StatusUnprocessableEntity, logMessage, timeStamp)
}

func BadGatewayErrorHandler(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	logMessage string,
	timeStamp time.Time,
) {
	aulogging.Logger.Ctx(ctx).Warn().Printf("bad gateway: %s", logMessage)
	errorHandler(ctx, w, r, "downstream.unavailable", http.StatusBadGateway,
		"a downstream web is currently unavailable or failed to service the request", timeStamp)
}

func UnexpectedErrorHandler(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	logMessage string,
	timeStamp time.Time,
) {
	aulogging.Logger.Ctx(ctx).Error().Printf("unexpected: %s", logMessage)
	errorHandler(ctx, w, r, "unknown", http.StatusInternalServerError, logMessage, timeStamp)
}

func errorHandler(
	ctx context.Context,
	w http.ResponseWriter,
	_ *http.Request,
	msg string,
	status int,
	details string,
	timestamp time.Time,
) {
	detailsPtr := &details
	if details == "" {
		detailsPtr = nil
	}
	response := &apimodel.Error{
		Details:   detailsPtr,
		Message:   &msg,
		Timestamp: &timestamp,
	}
	w.Header().Set(headers.ContentType, media.ContentTypeApplicationJson)
	w.WriteHeader(status)
	WriteJSON(ctx, w, response)
}

// --- helpers

func WriteJSON(ctx context.Context, w http.ResponseWriter, v any) {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(v)
	if err != nil {
		aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("error while encoding json response: %v", err)
		// can't change status anymore, in the middle of the response now
	}
}

func WriteYAML(ctx context.Context, w http.ResponseWriter, v any) {
	encoder := yaml.NewEncoder(w)
	err := encoder.Encode(v)
	if err != nil {
		aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("error while encoding yaml response: %v", err)
		// can't change status anymore, in the middle of the response now
	}
}

func ParseBody(_ context.Context, body io.ReadCloser, resource any) error {
	jsonBytes, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, resource)
}
