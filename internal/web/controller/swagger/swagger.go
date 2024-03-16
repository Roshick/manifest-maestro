package swagger

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	webhelper "github.com/Roshick/manifest-maestro/internal/web/helper"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	auwebswaggerui "github.com/StephanHCB/go-autumn-web-swagger-ui"
	"github.com/go-chi/chi/v5"
	"github.com/go-http-utils/headers"
)

// ToDo: Rework

type SpecFile struct {
	RelativeFilesystemPath string
	FileName               string
	UriPath                string
}

func New() *Controller {
	return &Controller{}
}

type Controller struct {
}

func (c *Controller) WireUp(ctx context.Context, router chi.Router, additionalSpecFiles ...SpecFile) {
	c.AddStaticHttpFilesystemRoute(router, auwebswaggerui.Assets, "/swagger-ui")
	openApiSpecFile, fileFindError := c.GetFirstMatchingServiceableFile([]string{"docs", "api"}, regexp.MustCompile(`openapi-v3-spec\.(json|yaml)`))
	if fileFindError != nil {
		aulogging.Logger.NoCtx().Error().Print("failed to find openAPI spec file. OpenAPI spec will be unavailable.")
	}
	if err := c.AddStaticFileRoute(router, openApiSpecFile); fileFindError == nil && err != nil {
		aulogging.Logger.NoCtx().Error().Printf("failed to read openAPI spec file %s/%s. OpenAPI spec will be unavailable.", openApiSpecFile.RelativeFilesystemPath, openApiSpecFile.FileName)
	}

	for _, additionalFile := range additionalSpecFiles {
		if err := c.AddStaticFileRoute(router, additionalFile); fileFindError == nil && err != nil {
			aulogging.Logger.NoCtx().Error().Printf("failed to read spec file %s/%s. OpenAPI spec will be broken.", additionalFile.RelativeFilesystemPath, additionalFile.FileName)
		}
	}

	c.AddRedirect(router, "/v3/api-docs", fmt.Sprintf("/%s", openApiSpecFile.FileName))
}

func (c *Controller) AddStaticHttpFilesystemRoute(server chi.Router, fs http.FileSystem, uriPath string) {
	strippedFs := http.StripPrefix(uriPath, http.FileServer(fs))

	if hasNoTrailingSlash(uriPath) {
		server.Get(uriPath, http.RedirectHandler(uriPath+"/", 301).ServeHTTP)
		uriPath += "/"
	}
	uriPath += "*"

	server.Get(uriPath, func(w http.ResponseWriter, r *http.Request) {
		strippedFs.ServeHTTP(w, r)
	})
}

func (c *Controller) AddStaticFileRoute(server chi.Router, specFile SpecFile) error {
	workDir, _ := os.Getwd()
	filePath := filepath.Join(workDir, specFile.RelativeFilesystemPath, specFile.FileName)

	contents, err := os.ReadFile(filePath)
	if err != nil {
		aulogging.Logger.NoCtx().Info().WithErr(err).Printf("failed to read file %s - skipping: %s", filePath, err.Error())
		return err
	}

	if hasNoTrailingSlash(specFile.UriPath) {
		specFile.UriPath = specFile.UriPath + "/"
	}

	server.Get(specFile.UriPath+specFile.FileName, func(w http.ResponseWriter, r *http.Request) {
		// this stops browsers from caching our swagger.json
		w.Header().Set(headers.CacheControl, "no-cache")
		w.Header().Set(headers.ContentType, webhelper.ContentTypeApplicationJSON)
		_, _ = w.Write(contents)
	})

	return nil
}

func (c *Controller) GetFirstMatchingServiceableFile(relativeFilesystemPaths []string, fileMatcher *regexp.Regexp) (SpecFile, error) {
	workDir, _ := os.Getwd()
	for _, relativeFilesystemPath := range relativeFilesystemPaths {
		dirPath := filepath.Join(workDir, relativeFilesystemPath)

		contents, err := os.ReadDir(dirPath)
		if err != nil {
			aulogging.Logger.NoCtx().Info().WithErr(err).Printf("failed to read directory %s - skipping directory", dirPath)
			continue
		}

		for _, element := range contents {
			if !element.IsDir() && fileMatcher != nil && fileMatcher.MatchString(element.Name()) {
				return SpecFile{
					RelativeFilesystemPath: relativeFilesystemPath,
					FileName:               element.Name(),
				}, nil
			}
		}

	}
	return SpecFile{}, fmt.Errorf("no file matching %s found in relative paths %s", fileMatcher.String(), strings.Join(relativeFilesystemPaths, ", "))
}

func hasNoTrailingSlash(path string) bool {
	return len(path) == 0 || path[len(path)-1] != '/'
}

func (c *Controller) AddRedirect(server chi.Router, source string, target string) {
	server.Get(source, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
}
