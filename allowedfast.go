package torrent

import (
	"crypto/sha1"
	"encoding/binary"
	"net"

	"github.com/anacrolix/torrent/metainfo"
	pp "github.com/anacrolix/torrent/peer_protocol"
)

// allowedFastPieces is the BEP 6 allowed-fast set for an IPv4 peer.
// IPv6 is skipped: TorrServer leaves IPv6 off by default, and the set is only a hint.
func allowedFastPieces(infoHash metainfo.Hash, ip net.IP, n, k int) []int {
	ip4 := ip.To4()
	if ip4 == nil || n <= 0 || k <= 0 {
		return nil
	}
	masked := append(net.IP(nil), ip4...)
	masked[3] = 0
	x := make([]byte, 0, 24)
	x = append(x, infoHash[:]...)
	x = append(x, masked...)
	out := make([]int, 0, k)
	seen := make(map[int]struct{}, k)
	for len(out) < k && len(out) < n {
		sum := sha1.Sum(x)
		x = sum[:]
		for i := 0; i < 5 && len(out) < k && len(out) < n; i++ {
			j := int(binary.BigEndian.Uint32(sum[i*4:(i+1)*4])) % n
			if _, ok := seen[j]; ok {
				continue
			}
			seen[j] = struct{}{}
			out = append(out, j)
		}
	}
	return out
}

func (cn *connection) sendAllowedFast() {
	t := cn.t
	if !cn.fastEnabled() || !t.haveInfo() {
		return
	}
	for _, idx := range allowedFastPieces(t.infoHash, cn.remoteAddr.IP, t.numPieces(), 10) {
		if !t.havePiece(idx) {
			continue
		}
		cn.Post(pp.Message{Type: pp.AllowedFast, Index: pp.Integer(idx)})
	}
}
