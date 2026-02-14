package torrent

import (
	"fmt"
	"sync/atomic"
	"time"
)

type Stats struct {
	TotalSize            int64
	PexProcessed         atomic.Int32
	PexAdded             atomic.Int32
	PexSent              atomic.Int32
	PeersProcessed       atomic.Int32
	PeersConfirmed       atomic.Int32
	PeersDenied          atomic.Int32
	StartTime            time.Time
	GlobalBitfield       Bitfield
	TotalWritten         int64
	CurrentlyDownloading atomic.Int32
	Failed               atomic.Int32
	NumPeers             atomic.Int32
	Searching            atomic.Int32
	NotFound             atomic.Int32
	UnchokedPeers        atomic.Int32
	Seeders              atomic.Int32
	BitfieldRecv         atomic.Int32
	BitfieldMiss         atomic.Int32
	ValidTrackers        atomic.Int32
	PeersProvided        atomic.Int32
}

func (d *Downloader) printStats() {
	elapsed := time.Since(d.Stats.StartTime).Seconds()
	var avgSpeed float64
	if elapsed > 0 {
		avgSpeed = float64(d.Stats.TotalWritten) / elapsed
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
PEX Sent:       %-8d | Peers Provided: %-8d
Peers Proc:     %-8d | Peers Confirm:  %-8d 
Peers Denied:   %-8d

BITFIELD & ERRORS
---------------------------------------------------------
Bitfield Recv: %-8d | Bitfield Miss: %-8d
Failed:        %-8d | Not Found:     %-8d
=========================================================
`,
		d.piecesDone, len(d.tf.PieceHashes),
		formatBytes(float64(d.Stats.TotalWritten)),
		formatBytes(avgSpeed),
		time.Since(d.Stats.StartTime).Round(time.Second),

		d.Stats.ValidTrackers.Load(), d.Stats.NumPeers.Load(),
		d.Stats.UnchokedPeers.Load(), d.Stats.Seeders.Load(),
		d.Stats.CurrentlyDownloading.Load(), d.Stats.Searching.Load(),

		d.Stats.PexProcessed.Load(), d.Stats.PexAdded.Load(),
		d.Stats.PexSent.Load(), d.Stats.PeersProvided.Load(),
		d.Stats.PeersProcessed.Load(), d.Stats.PeersConfirmed.Load(),
		d.Stats.PeersDenied.Load(),

		d.Stats.BitfieldRecv.Load(), d.Stats.BitfieldMiss.Load(),
		d.Stats.Failed.Load(), d.Stats.NotFound.Load(),
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
