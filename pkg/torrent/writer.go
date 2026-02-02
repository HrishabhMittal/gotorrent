package torrent
import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type TorrentWriter struct {
	tf *TorrentFile
	mu sync.Mutex
}

func NewTorrentWriter(tf *TorrentFile) (*TorrentWriter, error) {
	for _, f := range tf.Files {
		dir := filepath.Dir(f.Path)
		if dir != "." && dir != "/" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory %s: %v", dir, err)
			}
		}

		file, err := os.OpenFile(f.Path, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %v", f.Path, err)
		}

		if err := file.Truncate(int64(f.Length)); err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to allocate space for %s: %v", f.Path, err)
		}
		file.Close()
	}

	return &TorrentWriter{
		tf: tf,
	}, nil
}

func (w *TorrentWriter) Write(index int, begin int, data []byte) error {
	globalOffset := int64(index)*int64(w.tf.PieceLength) + int64(begin)
	bytesToWrite := len(data)

	currentFileStart := int64(0)

	for _, f := range w.tf.Files {
		fileLen := int64(f.Length)
		fileEnd := currentFileStart + fileLen

		if globalOffset >= currentFileStart && globalOffset < fileEnd {
			relativeOffset := globalOffset - currentFileStart
			amount := int64(bytesToWrite)
			if globalOffset+amount > fileEnd {
				amount = fileEnd - globalOffset
			}
			err := w.writeToFile(f.Path, data[:amount], relativeOffset)
			if err != nil {
				return err
			}

			globalOffset += amount
			bytesToWrite -= int(amount)
			data = data[amount:]

			if bytesToWrite == 0 {
				return nil
			}
		}
		currentFileStart += fileLen
	}

	if bytesToWrite > 0 {
		return fmt.Errorf("wrote everything but still had %d bytes left (file size mismatch?)", bytesToWrite)
	}

	return nil
}
func (w *TorrentWriter) Read(index int, begin int, length int) ([]byte, error) {
	globalOffset := int64(index)*int64(w.tf.PieceLength) + int64(begin)
	buf := make([]byte, length)

	currentFileStart := int64(0)
	bytesReadTotal := 0

	for _, f := range w.tf.Files {
		fileLen := int64(f.Length)
		fileEnd := currentFileStart + fileLen

		if globalOffset >= currentFileStart && globalOffset < fileEnd {
			relativeOffset := globalOffset - currentFileStart

			amount := int64(length - bytesReadTotal)
			if globalOffset+amount > fileEnd {
				amount = fileEnd - globalOffset
			}

			chunk, err := w.readFromFile(f.Path, relativeOffset, int(amount))
			if err != nil {
				return nil, err
			}

			copy(buf[bytesReadTotal:], chunk)

			globalOffset += amount
			bytesReadTotal += int(amount)

			if bytesReadTotal == length {
				return buf, nil
			}
		}
		currentFileStart += fileLen
	}

	if bytesReadTotal < length {
		return nil, fmt.Errorf("could only read %d bytes out of %d", bytesReadTotal, length)
	}

	return buf, nil
}

func (w *TorrentWriter) writeToFile(path string, data []byte, offset int64) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteAt(data, offset)
	return err
}

func (w *TorrentWriter) readFromFile(path string, offset int64, length int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, length)
	n, err := f.ReadAt(buf, offset)
	if err != nil && n != length {
		return nil, err
	}
	return buf, nil
}
