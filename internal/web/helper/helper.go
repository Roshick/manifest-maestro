package helper

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Roshick/manifest-maestro/internal/web/header"
	"github.com/Roshick/manifest-maestro/internal/web/mimetype"

	"github.com/Roshick/manifest-maestro/pkg/api"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"
)

// ToDo rework

// --- success handlers ---

func Success(ctx context.Context, w http.ResponseWriter, r *http.Request, response any) {
	acceptEncodingHeader := r.Header.Get(header.Accept)
	if strings.Contains(acceptEncodingHeader, mimetype.ApplicationYAML) {
		w.Header().Set(header.ContentType, mimetype.ApplicationYAML)
		WriteYAML(ctx, w, response)
	} else {
		w.Header().Set(header.ContentType, mimetype.ApplicationJSON)
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
	response := &api.ErrorResponse{}
	w.Header().Set(header.ContentType, mimetype.ApplicationJSON)
	w.WriteHeader(status)
	WriteJSON(ctx, w, response)
}

// --- helpers

func StringPathParam(r *http.Request, key string) (string, error) {
	value := chi.URLParam(r, key)
	return url.PathUnescape(value)
}

func StringQueryParam(r *http.Request, key string, defaultValue string) (string, error) {
	query := r.URL.Query()
	value := query.Get(key)
	if value == "" {
		return defaultValue, nil
	}
	return url.QueryUnescape(value)
}

func BooleanQueryParam(r *http.Request, key string, defaultValue bool) (bool, error) {
	query := r.URL.Query()
	value := query.Get(key)
	if value == "" {
		return defaultValue, nil
	}
	return strconv.ParseBool(query.Get(key))
}

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
