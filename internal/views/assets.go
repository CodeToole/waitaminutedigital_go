package views

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	assetVersionOnce sync.Once
	assetVersion     string
	assetVersionErr  error
)

// InitializeAssetVersion chooses BUILD_VERSION when supplied, otherwise
// hashes all static files once. main calls this before accepting requests.
func InitializeAssetVersion(staticDir string) error {
	assetVersionOnce.Do(func() {
		if version := strings.TrimSpace(os.Getenv("BUILD_VERSION")); version != "" {
			assetVersion = version
			return
		}
		assetVersion, assetVersionErr = hashStaticDirectory(staticDir)
	})
	return assetVersionErr
}

// AssetURL adds the deployment's shared content version to a local static
// URL. Keeping this in one helper prevents templates from drifting back to
// unversioned, long-cached asset paths.
func AssetURL(assetPath string) string {
	if !strings.HasPrefix(assetPath, "/static/") {
		return assetPath
	}
	if err := InitializeAssetVersion("static"); err != nil {
		return assetPath
	}
	parsed, err := url.Parse(assetPath)
	if err != nil {
		return assetPath
	}
	query := parsed.Query()
	query.Set("v", assetVersion)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func hashStaticDirectory(root string) (string, error) {
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(hash, "%s\x00", filepath.ToSlash(relative)); err != nil {
			return err
		}
		_, err = hash.Write(content)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("hash static assets: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil))[:16], nil
}
