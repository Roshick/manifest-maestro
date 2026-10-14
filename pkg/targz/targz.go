package targz

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/Roshick/manifest-maestro/pkg/filesystem"
	"github.com/go-git/go-billy/v5"

	aulogging "github.com/StephanHCB/go-autumn-logging"
)

func Compress(
	ctx context.Context,
	fileSystem *filesystem.FileSystem,
	sourcePath string,
	targetSubPath string,
	targetWriter io.Writer,
) error {
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
			_ = file.Close()
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
	defer func() {
		if innerErr := gzr.Close(); innerErr != nil {
			aulogging.Logger.Ctx(ctx).Warn().WithErr(innerErr).Printf("failed to close gzip reader")
		}
	}()
	tr := tar.NewReader(gzr)

	var buf []byte
	for {
		header, innerErr := tr.Next()
		if errors.Is(innerErr, io.EOF) {
			break
		}
		if innerErr != nil {
			return innerErr
		}

		filePath := fileSystem.Join(targetPath, header.Name)
		if filePath != targetPath &&
			!strings.HasPrefix(filePath, strings.TrimSuffix(targetPath, fileSystem.Separator)+fileSystem.Separator) {
			return fmt.Errorf(
				"failed to extract file '%s': path escapes target directory '%s'",
				header.Name,
				targetPath,
			)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
			if buf, innerErr = extractRegularFile(fileSystem, tr, header.Size, filePath, buf); innerErr != nil {
				return innerErr
			}
		default:
			return fmt.Errorf("failed to extract file '%s' of unsupported type from '%s'", header.Name, targetPath)
		}
	}

	return nil
}

// extractRegularFile reads the current tar entry into buf (grown as needed) and returns it for reuse.
// Reading the exact size and writing once avoids a copy buffer per file and repeated slice growth.
func extractRegularFile(
	fileSystem *filesystem.FileSystem, tr *tar.Reader, size int64, filePath string, buf []byte,
) ([]byte, error) {
	if err := fileSystem.MkdirAll(fileSystem.Dir(filePath)); err != nil {
		return buf, err
	}
	if int64(cap(buf)) < size {
		buf = make([]byte, size)
	}
	buf = buf[:size]
	if _, err := io.ReadFull(tr, buf); err != nil {
		return buf, err
	}
	return buf, fileSystem.WriteFile(filePath, buf)
}

// CompressFromBilly compresses a billy filesystem directly into a tar.gz stream,
// avoiding an intermediate copy to an in-memory filesystem.
func CompressFromBilly(
	ctx context.Context,
	billyFS billy.Filesystem,
	targetWriter io.Writer,
) error {
	gzw := gzip.NewWriter(targetWriter)
	defer func() {
		if err := gzw.Close(); err != nil {
			aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to close gzip writer")
		}
	}()
	tw := tar.NewWriter(gzw)
	defer func() {
		if err := tw.Close(); err != nil {
			aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to close tar writer")
		}
	}()

	return compressBillyDir(ctx, billyFS, tw, "")
}

func compressBillyDir(ctx context.Context, billyFS billy.Filesystem, tw *tar.Writer, currentPath string) error {
	files, err := billyFS.ReadDir(currentPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		filePath := currentPath
		if filePath == "" {
			filePath = file.Name()
		} else {
			filePath = filePath + "/" + file.Name()
		}

		if file.IsDir() {
			if innerErr := compressBillyDir(ctx, billyFS, tw, filePath); innerErr != nil {
				return innerErr
			}
			continue
		}

		header, innerErr := tar.FileInfoHeader(file, file.Name())
		if innerErr != nil {
			return innerErr
		}
		header.Name = filePath

		if innerErr = tw.WriteHeader(header); innerErr != nil {
			return innerErr
		}

		src, innerErr := billyFS.Open(filePath)
		if innerErr != nil {
			return innerErr
		}
		if _, innerErr = io.Copy(tw, src); innerErr != nil {
			_ = src.Close()
			return innerErr
		}
		if innerErr = src.Close(); innerErr != nil {
			aulogging.Logger.Ctx(ctx).Warn().WithErr(innerErr).Printf("failed to close '%s'", filePath)
		}
	}
	return nil
}
