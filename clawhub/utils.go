package clawhub

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	goversion "github.com/hashicorp/go-version"
)

// CalculateHash calculates the SHA256 hash of a directory
func CalculateHash(dirPath string) (string, error) {
	hash := sha256.New()

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and hidden files
		if info.IsDir() {
			// Skip .git directory and hidden files
			if strings.HasPrefix(filepath.Base(path), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}

		// Read file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Write relative path and data to hash
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}

		hash.Write([]byte(relPath))
		hash.Write(data)

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

// CreateZipBundle creates a zip file from a directory
func CreateZipBundle(dirPath string) ([]byte, error) {
	buf := new(bytes.Buffer)

	zipWriter := zip.NewWriter(buf)

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and hidden files
		if info.IsDir() {
			// Skip .git directory
			if strings.HasPrefix(filepath.Base(path), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}

		// Read file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}

		// Create file in zip
		header := &zip.FileHeader{
			Name:    filepath.ToSlash(relPath),
			Method:  zip.Deflate,
			Modified: info.ModTime(),
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		_, err = writer.Write(data)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		zipWriter.Close()
		return nil, fmt.Errorf("failed to create zip: %w", err)
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// ExtractZipBundle extracts a zip file to a directory
func ExtractZipBundle(data []byte, destDir string) error {
	reader := bytes.NewReader(data)
	zipReader, err := zip.NewReader(reader, reader.Size())
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}

	for _, file := range zipReader.File {
		// Sanitize file path to prevent directory traversal
		filePath := filepath.Join(destDir, file.Name)

		// Check for path traversal
		if !strings.HasPrefix(filePath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", file.Name)
		}

		// Create directory for file
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Extract file or directory
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		// Open file in zip
		fileReader, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open file in zip: %w", err)
		}

		// Create file
		destFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			fileReader.Close()
			return fmt.Errorf("failed to create file: %w", err)
		}

		// Copy file data
		if _, err := io.Copy(destFile, fileReader); err != nil {
			fileReader.Close()
			destFile.Close()
			return fmt.Errorf("failed to write file: %w", err)
		}

		fileReader.Close()
		destFile.Close()
	}

	return nil
}

// BumpVersion bumps a semver version
func BumpVersion(currentVersion string, bumpType string) (string, error) {
	v, err := goversion.NewVersion(currentVersion)
	if err != nil {
		return "", fmt.Errorf("invalid version: %w", err)
	}

	segments := v.Segments()
	if len(segments) < 3 {
		return "", fmt.Errorf("invalid semver format: %s", currentVersion)
	}

	major, minor, patch := segments[0], segments[1], segments[2]

	switch bumpType {
	case "major":
		major++
		minor = 0
		patch = 0
	case "minor":
		minor++
		patch = 0
	case "patch":
		patch++
	default:
		return "", fmt.Errorf("invalid bump type: %s (must be major, minor, or patch)", bumpType)
	}

	newVersion, err := goversion.NewVersion(fmt.Sprintf("%d.%d.%d", major, minor, patch))
	if err != nil {
		return "", fmt.Errorf("failed to create new version: %w", err)
	}

	return newVersion.Original(), nil
}

// CompareVersions compares two versions
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func CompareVersions(v1, v2 string) (int, error) {
	ver1, err := goversion.NewVersion(v1)
	if err != nil {
		return 0, fmt.Errorf("invalid version v1: %w", err)
	}

	ver2, err := goversion.NewVersion(v2)
	if err != nil {
		return 0, fmt.Errorf("invalid version v2: %w", err)
	}

	return ver1.Compare(ver2), nil
}

// ValidateSlug validates a skill slug
func ValidateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("slug cannot be empty")
	}

	// Slug should only contain lowercase letters, numbers, and hyphens
	for _, c := range slug {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return fmt.Errorf("slug can only contain lowercase letters, numbers, hyphens, and underscores")
		}
	}

	if strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") {
		return fmt.Errorf("slug cannot start or end with a hyphen")
	}

	if len(slug) < 2 || len(slug) > 50 {
		return fmt.Errorf("slug must be between 2 and 50 characters")
	}

	return nil
}

// ValidateVersion validates a semver version
func ValidateVersion(v string) error {
	_, err := goversion.NewVersion(v)
	if err != nil {
		return fmt.Errorf("invalid semver version: %w", err)
	}
	return nil
}

// ValidateSkillDir validates that a directory is a valid skill
func ValidateSkillDir(dirPath string) error {
	// Check if directory exists
	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("skill directory does not exist: %s", dirPath)
		}
		return fmt.Errorf("failed to access skill directory: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("skill path is not a directory: %s", dirPath)
	}

	// Check for SKILL.md file
	skillFile := filepath.Join(dirPath, "SKILL.md")
	if _, err := os.Stat(skillFile); os.IsNotExist(err) {
		return fmt.Errorf("SKILL.md not found in skill directory")
	}

	return nil
}

// FindSkillDirectories finds all directories containing SKILL.md
func FindSkillDirectories(roots []string) ([]string, error) {
	var skillDirs []string

	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				// Skip inaccessible directories
				if os.IsPermission(err) {
					return nil
				}
				return err
			}

			// Skip hidden directories
			if info.IsDir() && strings.HasPrefix(filepath.Base(path), ".") {
				return filepath.SkipDir
			}

			// Check for SKILL.md
			if !info.IsDir() && info.Name() == "SKILL.md" {
				skillDirs = append(skillDirs, filepath.Dir(path))
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to scan directory %s: %w", root, err)
		}
	}

	return skillDirs, nil
}

// SanitizePath sanitizes a file path to prevent directory traversal
func SanitizePath(basePath, inputPath string) (string, error) {
	// Clean the input path
	cleanPath := filepath.Clean(inputPath)

	// Join with base path
	fullPath := filepath.Join(basePath, cleanPath)

	// Ensure the result is within the base path
	relPath, err := filepath.Rel(basePath, fullPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if strings.HasPrefix(relPath, "..") {
		return "", fmt.Errorf("path traversal detected: %s", inputPath)
	}

	return fullPath, nil
}
