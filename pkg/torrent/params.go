package torrent

import "time"

const (
	PIECE_QUEUE          = 256
	PEX_CHANNEL          = 512
	DISCOVERY_LIMIT      = 64
	CONFIRMED_PEER_QUEUE = 812
	HANDSHAKE_WAIT_TIMEOUT = 10
	READ_MSG_TIMEOUT = 130
	REQUEST_BLOCK_SIZE   = 16384
	MAX_CHOKED_TIME      = 60 * time.Second
	MAX_BACKLOG          = 32
	MAX_MSG_LEN          = 262144
	PEX_INTERVAL         = 60 * time.Second
)
