// Package filesystem provides adapters for filesystem operations.
package filesystem

import (
	"os"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/service"
)

// fileInfoAdapter adapts os.FileInfo to service.FileInfo interface.
type fileInfoAdapter struct {
	info os.FileInfo
}

// NewFileInfoAdapter creates a new FileInfo adapter from os.FileInfo.
func NewFileInfoAdapter(info os.FileInfo) service.FileInfo {
	return &fileInfoAdapter{info: info}
}

func (f *fileInfoAdapter) Name() string {
	return f.info.Name()
}

func (f *fileInfoAdapter) Size() int64 {
	return f.info.Size()
}

func (f *fileInfoAdapter) Mode() uint32 {
	return uint32(f.info.Mode())
}

func (f *fileInfoAdapter) ModTime() time.Time {
	return f.info.ModTime()
}

func (f *fileInfoAdapter) IsDir() bool {
	return f.info.IsDir()
}






