package torrent

import (
	"bytes"
	"crypto/sha1"
	"fmt"
)

func Verify(tf *TorrentFile) error {
	w, err := NewTorrentWriter(tf,nil)
	if err != nil {
		return fmt.Errorf("failed to initialize writer: %v", err)
	}
	fmt.Printf("Verifying %d pieces...\n", len(tf.PieceHashes))
	for i, expectedHash := range tf.PieceHashes {
		pieceLen := tf.PieceLength
		if i == len(tf.PieceHashes)-1 {
			pieceLen = tf.Length - (i * tf.PieceLength)
		}
		data, err := w.Read(i, 0, pieceLen)
		if err != nil {
			return fmt.Errorf("failed to read piece %d: %v (file missing or corrupt?)", i, err)
		}
		hash := sha1.Sum(data)
		if !bytes.Equal(hash[:], expectedHash[:]) {
			return fmt.Errorf("verification failed at piece %d: hash mismatch", i)
		}
		fmt.Printf("\rVerified: %d/%d pieces", i+1, len(tf.PieceHashes))
	}
	fmt.Println("\nVerification successful! Files are intact.")
	return nil
}
