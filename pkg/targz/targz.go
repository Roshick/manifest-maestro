package targz

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/Roshick/manifest-maestro/pkg/filesystem"
	"io"
	"io/fs"
	"strings"

	aulogging "github.com/StephanHCB/go-autumn-logging"
)

func Compress(ctx context.Context, fileSystem *filesystem.FileSystem, sourcePath string, targetSubPath string, targetWriter io.Writer) error {
	if !fileSystem.Exists(sourcePath) {
		return fmt.Errorf("file at '%s' does not exist", sourcePath)
	}

	gzw := gzip.NewWriter(targetWriter)
	defer func() {
		if err := gzw.Close(); err != nil {
			aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to close gzip writer for '%s'", sourcePath)
		}
	}()
	tw := tar.NewWriter(gzw)
	defer func() {
		if err := tw.Close(); err != nil {
			aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to close tar writer for '%s'", sourcePath)
		}
	}()

	return fileSystem.Walk(sourcePath, func(filePath string, fileInfo fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fileInfo.Mode().IsRegular() {
			return fmt.Errorf("cannot compress irregular file '%s'", filePath)
		}
		if fileInfo.IsDir() {
			return nil
		}

		header, err := tar.FileInfoHeader(fileInfo, fileInfo.Name())
		if err != nil {
			return err
		}
		header.Name = strings.TrimPrefix(filePath, sourcePath)
		header.Name = strings.TrimPrefix(header.Name, fileSystem.Separator)
		header.Name = fileSystem.Join(targetSubPath, header.Name)
		if err = tw.WriteHeader(header); err != nil {
			return err
		}

		file, err := fileSystem.Open(filePath)
		if err != nil {
			return err
		}
		if _, err = io.Copy(tw, file); err != nil {
			return err
		}
		if err = file.Close(); err != nil {
			aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to close '%s'", filePath)
		}
		return nil
	})
}

func Extract(ctx context.Context, fileSystem *filesystem.FileSystem, sourceReader io.Reader, targetPath string) error {
	if err := fileSystem.MkdirAll(targetPath); err != nil {
		return err
	}

	gzr, err := gzip.NewReader(sourceReader)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gzr)

	for {
		header, innerErr := tr.Next()
		if innerErr == io.EOF {
			break
		}
		if innerErr != nil {
			return innerErr
		}

		filePath := fileSystem.Join(targetPath, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
			if innerErr2 := fileSystem.MkdirAll(fileSystem.Dir(filePath)); innerErr2 != nil {
				return innerErr2
			}
			file, innerErr2 := fileSystem.Create(filePath)
			if innerErr2 != nil {
				return innerErr2
			}
			if _, innerErr2 = io.Copy(file, tr); innerErr2 != nil {
				return innerErr2
			}
			if innerErr2 = file.Close(); innerErr2 != nil {
				aulogging.Logger.Ctx(ctx).Warn().WithErr(innerErr2).Printf("failed to close '%s'", filePath)
			}
		default:
			return fmt.Errorf("failed to extract file '%s' of unsupported type from '%s'", header.Name, targetPath)
		}
	}

	return nil
}
