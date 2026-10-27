package torrent

import (
	"net"
	"testing"

	"github.com/anacrolix/torrent/metainfo"
)

func TestAllowedFastStableAndMasked(t *testing.T) {
	var ih metainfo.Hash
	ih[0] = 0x42
	a := allowedFastPieces(ih, net.IPv4(1, 2, 3, 4).To4(), 100, 10)
	b := allowedFastPieces(ih, net.IPv4(1, 2, 3, 200).To4(), 100, 10)
	if len(a) != 10 {
		t.Fatalf("len %d", len(a))
	}
	seen := map[int]struct{}{}
	for _, p := range a {
		if p < 0 || p >= 100 {
			t.Fatalf("piece %d out of range", p)
		}
		if _, ok := seen[p]; ok {
			t.Fatalf("duplicate %d", p)
		}
		seen[p] = struct{}{}
	}
	if len(b) != len(a) {
		t.Fatal("length mismatch")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("last octet should not change the set")
		}
	}
	ih[1] = 1
	c := allowedFastPieces(ih, net.IPv4(1, 2, 3, 4).To4(), 100, 10)
	same := true
	for i := range a {
		if a[i] != c[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("infohash should change the set")
	}
}

func TestAllowedFastSkipsIPv6(t *testing.T) {
	if allowedFastPieces(metainfo.Hash{}, net.ParseIP("2001:db8::1"), 10, 10) != nil {
		t.Fatal("ipv6 set")
	}
}
