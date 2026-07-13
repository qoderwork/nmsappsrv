package upgrade

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"

	"nmsappsrv/internal/config"
)

// uploadBaseDir returns the configured local directory for upgrade file storage.
// Falls back to ./data/upgrade-files when not configured.
func uploadBaseDir() string {
	if config.Cfg != nil && config.Cfg.Upgrade.UploadDir != "" {
		return config.Cfg.Upgrade.UploadDir
	}
	return "./data/upgrade-files"
}

// saveUpgradeFile persists the given bytes under <uploadDir>/<tenancyId>/<uuid>_<name>
// and returns the absolute on-disk path. The path is what gets stored in
// upgrade_file.file_path and later served to devices via the file-server endpoint.
func saveUpgradeFile(tenancyId int, name string, data []byte) (string, error) {
	dir := uploadBaseDir()
	tenantDir := filepath.Join(dir, strconv.Itoa(tenancyId))
	if err := os.MkdirAll(tenantDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create upgrade storage dir: %w", err)
	}

	// Strip any directory components from the supplied name to avoid path traversal.
	safeName := filepath.Base(name)
	if safeName == "" || safeName == "." || safeName == string(os.PathSeparator) {
		safeName = "firmware.bin"
	}

	storedName := fmt.Sprintf("%s_%s", replaceHyphens(uuid.New().String()), safeName)
	storedPath := filepath.Join(tenantDir, storedName)

	if err := os.WriteFile(storedPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write upgrade file: %w", err)
	}

	absPath, err := filepath.Abs(storedPath)
	if err != nil {
		return storedPath, nil
	}
	return absPath, nil
}
