package swagger

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Roshick/manifest-maestro/internal/web/header"
	"github.com/Roshick/manifest-maestro/internal/web/mimetype"

	aulogging "github.com/StephanHCB/go-autumn-logging"
	auwebswaggerui "github.com/StephanHCB/go-autumn-web-swagger-ui"
	"github.com/go-chi/chi/v5"
)

type SpecFile struct {
	RelativeFilesystemPath string
	FileName               string
	UriPath                string
}

func NewController() *Controller {
	return &Controller{}
}

type Controller struct {
}

func (c *Controller) WireUp(_ context.Context, router chi.Router, additionalSpecFiles ...SpecFile) {
	c.AddStaticHttpFilesystemRoute(router, auwebswaggerui.Assets, "/swagger-ui")
	openApiSpecFile, err := c.GetFirstMatchingServiceableFile([]string{"api"}, regexp.MustCompile(`openapi[.]yaml`))
	if err != nil {
		aulogging.Logger.NoCtx().Error().Print("failed to find OpenAPI specification file. OpenAPI specification will be unavailable.")
		return
	}

	if err = c.AddStaticFileRoute(router, openApiSpecFile); err != nil {
		aulogging.Logger.NoCtx().Error().Printf("failed to read OpenAPI specification file %s. OpenAPI specification will be unavailable.", filepath.Join(openApiSpecFile.RelativeFilesystemPath, openApiSpecFile.FileName))
		return
	}

	for _, additionalFile := range additionalSpecFiles {
		if err = c.AddStaticFileRoute(router, additionalFile); err != nil {
			aulogging.Logger.NoCtx().Error().Printf("failed to read additional OpenAPI specification file %s. OpenAPI specification will be broken.", filepath.Join(additionalFile.RelativeFilesystemPath, additionalFile.FileName))
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
		w.Header().Set(header.CacheControl, "no-gitrepositorycache")
		w.Header().Set(header.ContentType, mimetype.ApplicationJSON)
		_, _ = w.Write(contents)
	})

	return nil
}

func (c *Controller) GetFirstMatchingServiceableFile(relativeFilesystemPaths []string, fileMatcher *regexp.Regexp) (SpecFile, error) {
	if fileMatcher == nil {
		return SpecFile{}, fmt.Errorf("file matcher is nil")
	}

	workDir, _ := os.Getwd()
	for _, relativeFilesystemPath := range relativeFilesystemPaths {
		dirPath := filepath.Join(workDir, relativeFilesystemPath)

		contents, err := os.ReadDir(dirPath)
		if err != nil {
			aulogging.Logger.NoCtx().Info().WithErr(err).Printf("failed to read directory %s - skipping directory", dirPath)
			continue
		}

		for _, element := range contents {
			if !element.IsDir() && fileMatcher.MatchString(element.Name()) {
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
