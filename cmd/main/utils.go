package main

import "math/rand"


func genPeerID(fmt string,seed int64) string {
	out := make([]byte,len(fmt))
	random := rand.New(rand.NewSource(seed))
	for i,c := range fmt {
		if c=='X' {
			num := random.Intn(62)
			if (num<10) {
				out[i]=byte(num+int('0'))
				continue
			}
			num-=10
			if (num<26) {
				out[i]=byte(num+int('a'))
				continue
			}
			num-=26
			out[i]=byte(num+int('A'))
		} else {
			out[i]=byte(c)
		}
	}
	return string(out)
}
