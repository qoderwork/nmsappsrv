package filepiecemeal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"nmsappsrv/pkg/logger"
)

// Service implements Java's FilePiecemealUploadManagementService: large files
// (e.g. upgrade packages) are uploaded in fixed chunks, each chunk verified by
// SHA-256, then assembled and re-verified against the whole-file SHA-256.
//
// Note on field naming: Java's DTOs call the whole-file digest "md5" and the
// per-part digest "partMd5", but both are actually SHA-256 digests (Java
// verifies parts via SHA256Util). We keep the JSON tag names for client
// compatibility while computing SHA-256, exactly like Java.
type Service struct {
	tempDir string
}

// NewService creates the piecemeal service. tempDir is config.FileServerConfig.PiecemealTempDir.
func NewService(tempDir string) *Service {
	if tempDir != "" {
		_ = os.MkdirAll(tempDir, 0o755)
	}
	return &Service{tempDir: tempDir}
}

func (s *Service) partDir(fileID string) string { return filepath.Join(s.tempDir, fileID) }
func (s *Service) partPath(fileID string, index int) string {
	return filepath.Join(s.partDir(fileID), fmt.Sprintf("%s_%d", fileID, index))
}

// GetPiecemealFileId returns the fileId for a whole-file digest (the digest
// itself, per Java getPiecemealFileId which sets fileId = data.getMd5()).
func (s *Service) GetPiecemealFileId(hash string) string {
	return hash
}

// UploadPiecemealFile persists one chunk: tempDir/{fileId}/{fileId}_{index}.
func (s *Service) UploadPiecemealFile(fileID string, index int, fileName string, r io.Reader) error {
	if s.tempDir == "" {
		return fmt.Errorf("piecemeal temp dir not configured")
	}
	if fileID == "" || index < 1 || fileName == "" {
		return fmt.Errorf("invalid arguments: fileId=%q index=%d fileName=%q", fileID, index, fileName)
	}
	dir := s.partDir(fileID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	target := s.partPath(fileID, index)
	if _, err := os.Stat(target); err == nil {
		_ = os.Remove(target)
	}
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, r)
	return err
}

// CheckNeedUpload reports whether the given chunk must be (re)uploaded: true if
// the part file is missing or its SHA-256 differs from partHash.
func (s *Service) CheckNeedUpload(fileID string, index int, partHash string) (bool, error) {
	target := s.partPath(fileID, index)
	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return true, err
	}
	got, err := Sha256Hex(target)
	if err != nil {
		return true, err
	}
	return !equalHex(got, partHash), nil
}

// Assemble merges tempDir/{fileId}/{fileId}_1.._{total} into {fileName} in
// order and verifies the merged SHA-256 equals hash. On success the parts are
// removed. Returns the merged file path, or an error (digest mismatch /
// assembly failure).
func (s *Service) Assemble(fileID string, total int, fileName, hash string) (string, error) {
	if s.tempDir == "" {
		return "", fmt.Errorf("piecemeal temp dir not configured")
	}
	if fileID == "" || total < 1 || fileName == "" || hash == "" {
		return "", fmt.Errorf("invalid arguments")
	}
	dir := s.partDir(fileID)
	for i := 1; i <= total; i++ {
		if _, err := os.Stat(s.partPath(fileID, i)); err != nil {
			return "", fmt.Errorf("part %d missing", i)
		}
	}
	merged := filepath.Join(dir, fileName)
	if _, err := os.Stat(merged); err == nil {
		_ = os.Remove(merged)
	}
	out, err := os.Create(merged)
	if err != nil {
		return "", err
	}
	for i := 1; i <= total; i++ {
		p := s.partPath(fileID, i)
		f, err := os.Open(p)
		if err != nil {
			out.Close()
			return "", err
		}
		if _, err := io.Copy(out, f); err != nil {
			f.Close()
			out.Close()
			return "", err
		}
		f.Close()
	}
	out.Close()

	got, err := Sha256Hex(merged)
	if err != nil {
		return "", err
	}
	if !equalHex(got, hash) {
		logger.Errorf("piecemeal assemble digest mismatch: got %s want %s", got, hash)
		return "", fmt.Errorf("digest mismatch (code 10177)")
	}
	// cleanup parts
	for i := 1; i <= total; i++ {
		_ = os.Remove(s.partPath(fileID, i))
	}
	return merged, nil
}

func equalHex(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'a' && ca <= 'f' {
			ca -= 'a' - 'A'
		}
		if cb >= 'a' && cb <= 'f' {
			cb -= 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
