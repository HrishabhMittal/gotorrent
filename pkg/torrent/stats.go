package torrent

import (
	"fmt"
	"sync/atomic"
	"time"
)

type Stats struct {
	pexProcessed         atomic.Int32
	pexAdded             atomic.Int32
	peersProcessed       atomic.Int32
	peersConfirmed       atomic.Int32
	peersDenied          atomic.Int32
	startTime            time.Time
	globalBitfield       Bitfield
	totalWritten         int64
	currentlyDownloading atomic.Int32
	failed               atomic.Int32
	numPeers             atomic.Int32
	searching            atomic.Int32
	notFound             atomic.Int32
	unchokedPeers        atomic.Int32
	seeders              atomic.Int32
	BitfieldRecv         atomic.Int32
	BitfieldMiss         atomic.Int32
	validTrackers        atomic.Int32
	peersProvided        atomic.Int32
}

func (d *Downloader) printStats() {
	elapsed := time.Since(d.stats.startTime).Seconds()
	var avgSpeed float64
	if elapsed > 0 {
		avgSpeed = float64(d.stats.totalWritten) / elapsed
	}

	fmt.Print("\033[H\033[2J")

	fmt.Printf(`
Gotorrent Full Status Dashboard
=========================================================
PROGRESS & SPEED
---------------------------------------------------------
Pieces:      [%d/%d]
Downloaded:  %s (Total)
Avg Speed:   %s/s
Uptime:      %s

NETWORK & PEERS
---------------------------------------------------------
Trackers:    %-10d | Peers (Total): %-10d
Unchoked:    %-10d | Seeders:       %-10d
Active DL:   %-10d | Searching:     %-10d

PEX & DISCOVERY
---------------------------------------------------------
PEX Processed:  %-8d | PEX Added:     %-8d
Peers Provided: %-8d | Peers Proc:    %-8d
Peers Confirm:  %-8d | Peers Denied:  %-8d

BITFIELD & ERRORS
---------------------------------------------------------
Bitfield Recv: %-8d | Bitfield Miss: %-8d
Failed:        %-8d | Not Found:     %-8d
=========================================================
`,
		d.piecesDone, len(d.tf.PieceHashes),
		formatBytes(float64(d.stats.totalWritten)),
		formatBytes(avgSpeed),
		time.Since(d.stats.startTime).Round(time.Second),

		d.stats.validTrackers.Load(), d.stats.numPeers.Load(),
		d.stats.unchokedPeers.Load(), d.stats.seeders.Load(),
		d.stats.currentlyDownloading.Load(), d.stats.searching.Load(),

		d.stats.pexProcessed.Load(), d.stats.pexAdded.Load(),
		d.stats.peersProvided.Load(), d.stats.peersProcessed.Load(),
		d.stats.peersConfirmed.Load(), d.stats.peersDenied.Load(),

		d.stats.BitfieldRecv.Load(), d.stats.BitfieldMiss.Load(),
		d.stats.failed.Load(), d.stats.notFound.Load(),
	)
}
func formatBytes(b float64) string {
	units := []string{"B", "KB", "MB", "GB"}
	idx := 0
	for b >= 1024 && idx < len(units)-1 {
		b /= 1024
		idx++
	}
	return fmt.Sprintf("%.2f %s", b, units[idx])
}
