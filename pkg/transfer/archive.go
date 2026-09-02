package transfer

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// StreamTarGz streams a directory into an io.Writer as tar.gz
func StreamTarGz(srcDir string, writer io.Writer) error {
	gzWriter := gzip.NewWriter(writer)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	srcDir = filepath.Clean(srcDir)
	baseDir := filepath.Base(srcDir)

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		tarPath := filepath.ToSlash(filepath.Join(baseDir, relPath))

		var link string
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return fmt.Errorf("failed to create tar header for %s: %w", path, err)
		}

		header.Name = tarPath
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header for %s: %w", path, err)
		}

		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open %s: %w", path, err)
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return fmt.Errorf("failed to copy %s to tar: %w", path, err)
			}
		}

		return nil
	})
}

// ExtractTarGz extracts a streaming tar.gz into the destination directory
func ExtractTarGz(reader io.Reader, destDir string) error {
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed reading tar entry: %w", err)
		}

		// Prevent zip-slip vulnerability
		cleanName := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			return fmt.Errorf("illegal file path in archive: %s", header.Name)
		}

		targetPath := filepath.Join(destDir, cleanName)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", targetPath, err)
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return fmt.Errorf("failed to create destination file %s: %w", targetPath, err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed writing to %s: %w", targetPath, err)
			}
			outFile.Close()
		}
	}

	return nil
}

// GetDirectorySize computes total uncompressed size of directory
func GetDirectorySize(dirPath string) (int64, error) {
	var size int64
	err := filepath.Walk(dirPath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}
