package torrent
import (
	"math/rand/v2"
)

var id int64 = rand.Int64()
var second uint64 = 6969

func genPeerID(fmt string) string {
	out := make([]byte, len(fmt))
	var random *rand.Rand = rand.New(rand.NewPCG(uint64(id), uint64(second)))
	for i, c := range fmt {
		if c == 'X' {
			num := random.IntN(62)
			if num < 10 {
				out[i] = byte(num + int('0'))
				continue
			}
			num -= 10
			if num < 26 {
				out[i] = byte(num + int('a'))
				continue
			}
			num -= 26
			out[i] = byte(num + int('A'))
		} else {
			out[i] = byte(c)
		}
	}
	return string(out)
}
