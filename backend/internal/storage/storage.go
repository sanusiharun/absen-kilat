// Package storage handles saving uploaded files (photos) to disk with
// validation of type and size.
package storage

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MaxPhotoBytes is the maximum accepted photo size (8 MB).
const MaxPhotoBytes = 8 << 20

// allowedExt maps allowed photo extensions to their canonical form.
var allowedExt = map[string]string{
	".jpg":  ".jpg",
	".jpeg": ".jpg",
	".png":  ".png",
	".webp": ".webp",
}

// Store saves an uploaded photo into dir and returns a URL-safe relative
// path (e.g. "2026/08/ab12cd34-....jpg"). It rejects unknown extensions and
// oversized files.
func Store(dir string, file multipart.File, header *multipart.FileHeader) (string, error) {
	if header == nil {
		return "", fmt.Errorf("storage.Store: nil file header")
	}
	if header.Size > MaxPhotoBytes {
		return "", fmt.Errorf("storage.Store: file too large (%d bytes)", header.Size)
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	canonical, ok := allowedExt[ext]
	if !ok {
		return "", fmt.Errorf("storage.Store: unsupported file type %q", ext)
	}

	now := time.Now().UTC()
	relDir := filepath.Join(now.Format("2006"), now.Format("01"))
	absDir := filepath.Join(dir, relDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return "", fmt.Errorf("storage.Store: mkdir: %w", err)
	}

	name := uuid.NewString() + canonical
	absPath := filepath.Join(absDir, name)
	dst, err := os.Create(absPath)
	if err != nil {
		return "", fmt.Errorf("storage.Store: create: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		_ = os.Remove(absPath)
		return "", fmt.Errorf("storage.Store: copy: %w", err)
	}

	return filepath.ToSlash(filepath.Join(relDir, name)), nil
}
