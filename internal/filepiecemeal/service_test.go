package filepiecemeal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestPiecemealFlow(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)

	whole := []byte("hello-world-this-is-a-piecemeal-test-file-content")
	// split into 3 chunks
	chunks := [][]byte{
		whole[:7],
		whole[7:20],
		whole[20:],
	}
	wholeHash := hashOf(whole)

	fileID := svc.GetPiecemealFileId(wholeHash)
	assert.Equal(t, wholeHash, fileID)

	// upload each chunk
	for i, c := range chunks {
		err := svc.UploadPiecemealFile(fileID, i+1, "big.bin", bytes.NewReader(c))
		assert.NoError(t, err)
	}

	// each chunk should report needUpload=false now
	for i := range chunks {
		need, err := svc.CheckNeedUpload(fileID, i+1, hashOf(chunks[i]))
		assert.NoError(t, err)
		assert.False(t, need, "chunk %d should not need re-upload", i)
	}

	// a tampered chunk should report needUpload=true
	need, err := svc.CheckNeedUpload(fileID, 1, hashOf([]byte("tampered")))
	assert.NoError(t, err)
	assert.True(t, need)

	// assemble
	merged, err := svc.Assemble(fileID, len(chunks), "big.bin", wholeHash)
	assert.NoError(t, err)

	got, err := os.ReadFile(merged)
	assert.NoError(t, err)
	assert.Equal(t, whole, got)

	// parts should be cleaned up after a successful assemble
	_, err = os.Stat(filepath.Join(tempDir, fileID, fileID+"_1"))
	assert.True(t, os.IsNotExist(err))
}

func TestAssembleDigestMismatch(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)
	chunk := []byte("abcdef")
	_ = svc.UploadPiecemealFile("fid", 1, "f.bin", bytes.NewReader(chunk))
	_, err := svc.Assemble("fid", 1, "f.bin", hashOf([]byte("wrong")))
	assert.Error(t, err)
}
