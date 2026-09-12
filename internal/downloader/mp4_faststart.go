package downloader

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// ValidateMoovBeforeMdat verifies that the top-level MP4 moov atom appears
// before mdat. This is the layout required for progressive playback.
func ValidateMoovBeforeMdat(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open MP4: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat MP4: %w", err)
	}
	fileSize := info.Size()
	var moovOffset, mdatOffset int64 = -1, -1

	for offset := int64(0); offset+8 <= fileSize; {
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			return fmt.Errorf("seek MP4 atom: %w", err)
		}
		var header [16]byte
		if _, err := io.ReadFull(file, header[:8]); err != nil {
			return fmt.Errorf("read MP4 atom: %w", err)
		}
		atomSize := int64(binary.BigEndian.Uint32(header[:4]))
		headerSize := int64(8)
		if atomSize == 1 {
			if _, err := io.ReadFull(file, header[8:16]); err != nil {
				return fmt.Errorf("read extended MP4 atom: %w", err)
			}
			if extended := binary.BigEndian.Uint64(header[8:16]); extended > uint64(^uint64(0)>>1) {
				return fmt.Errorf("MP4 atom is too large")
			} else {
				atomSize = int64(extended)
			}
			headerSize = 16
		} else if atomSize == 0 {
			atomSize = fileSize - offset
		}
		if atomSize < headerSize || atomSize > fileSize-offset {
			return fmt.Errorf("invalid MP4 atom size at offset %d", offset)
		}

		switch string(header[4:8]) {
		case "moov":
			if moovOffset < 0 {
				moovOffset = offset
			}
		case "mdat":
			if mdatOffset < 0 {
				mdatOffset = offset
			}
		}
		offset += atomSize
	}

	if moovOffset < 0 || mdatOffset < 0 {
		return fmt.Errorf("MP4 is missing moov or mdat atom")
	}
	if moovOffset > mdatOffset {
		return fmt.Errorf("MP4 moov atom is after mdat")
	}
	return nil
}

// EnsureMoovBeforeMdat repairs an MP4 that is not faststart and atomically
// replaces it only after validating the remuxed output.
func EnsureMoovBeforeMdat(ctx context.Context, filePath string, onProgress func(int)) error {
	if err := ValidateMoovBeforeMdat(filePath); err == nil {
		return nil
	} else {
		log.Printf("⚠️  MP4 is not faststart (%v); remuxing before upload", err)
	}

	temporaryPath := filePath + ".faststart.mp4"
	backupPath := fmt.Sprintf("%s.pre-faststart-%d", filePath, time.Now().UnixNano())
	_ = os.Remove(temporaryPath)
	if err := RemuxWithFaststart(ctx, filePath, temporaryPath, onProgress); err != nil {
		return fmt.Errorf("repair MP4 faststart: %w", err)
	}
	if err := ValidateMoovBeforeMdat(temporaryPath); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("validate repaired MP4 faststart: %w", err)
	}
	if err := os.Rename(filePath, backupPath); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("backup MP4 before faststart replacement: %w", err)
	}
	if err := os.Rename(temporaryPath, filePath); err != nil {
		_ = os.Rename(backupPath, filePath)
		return fmt.Errorf("replace MP4 with faststart output: %w", err)
	}
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		log.Printf("⚠️  Cannot remove pre-faststart backup %s: %v", backupPath, err)
	}
	return nil
}
