package torrent

import (
	"fmt"
	"sync/atomic"
	"time"
)
type Stats struct {
	startTime time.Time
	globalBitfield Bitfield
	totalWritten int64
	currentlyDownloading atomic.Int32
	failed atomic.Int32
	numPeers atomic.Int32
	searching atomic.Int32
	notFound atomic.Int32
	unchokedPeers atomic.Int32
	seeders atomic.Int32
	BitfieldRecv atomic.Int32
	BitfieldMiss atomic.Int32
}

func formatBytes(b float64) string {
	units := []string{"B/s", "KB/s", "MB/s", "GB/s"}
	idx := 0
	for b >= 1024 && idx < len(units)-1 {
		b /= 1024
		idx++
	}
	return fmt.Sprintf("%.2f %s", b, units[idx])
}
