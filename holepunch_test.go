package torrent

import (
	"bytes"
	"net"
	"net/netip"
	"sync"
	"testing"

	pp "github.com/anacrolix/torrent/peer_protocol"
	"github.com/anacrolix/torrent/peer_protocol/utholepunch"
)

func holepunchFixture(ip net.IP, port uint16) *connection {
	var mu sync.Mutex
	c := &connection{
		writeBuffer: new(bytes.Buffer),
		remoteAddr:  IpPort{IP: ip, Port: port},
		outgoing:    true,
		PeerExtensionIDs: map[pp.ExtensionName]pp.ExtensionNumber{
			pp.ExtensionNameHolepunch: holepunchExtendedId,
		},
	}
	c.writerCond.L = &mu
	return c
}

func TestHolepunchRendezvousNotConnected(t *testing.T) {
	sender := holepunchFixture(net.IPv4(1, 2, 3, 4).To4(), 4000)
	tor := &Torrent{cl: &Client{}, conns: map[*connection]struct{}{sender: {}}}
	sender.t = tor
	target := netip.AddrPortFrom(netip.AddrFrom4([4]byte{5, 6, 7, 8}), 5000)
	if err := tor.handleUtHolepunch(utholepunch.Msg{MsgType: utholepunch.Rendezvous, AddrPort: target}, sender); err != nil {
		t.Fatal(err)
	}
	want, err := (&utholepunch.Msg{MsgType: utholepunch.Error, AddrPort: target, ErrCode: utholepunch.NotConnected}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(sender.writeBuffer.Bytes(), want) {
		t.Fatalf("missing not-connected payload in %x", sender.writeBuffer.Bytes())
	}
}

func TestHolepunchRendezvousRelaysConnect(t *testing.T) {
	sender := holepunchFixture(net.IPv4(1, 2, 3, 4).To4(), 4000)
	targetConn := holepunchFixture(net.IPv4(5, 6, 7, 8).To4(), 5000)
	tor := &Torrent{cl: &Client{}, conns: map[*connection]struct{}{sender: {}, targetConn: {}}}
	sender.t = tor
	targetConn.t = tor
	target := netip.AddrPortFrom(netip.AddrFrom4([4]byte{5, 6, 7, 8}), 5000)
	if err := tor.handleUtHolepunch(utholepunch.Msg{MsgType: utholepunch.Rendezvous, AddrPort: target}, sender); err != nil {
		t.Fatal(err)
	}
	toSender, _ := (&utholepunch.Msg{MsgType: utholepunch.Connect, AddrPort: target}).MarshalBinary()
	toTarget, _ := (&utholepunch.Msg{
		MsgType:  utholepunch.Connect,
		AddrPort: netip.AddrPortFrom(netip.AddrFrom4([4]byte{1, 2, 3, 4}), 4000),
	}).MarshalBinary()
	if !bytes.Contains(sender.writeBuffer.Bytes(), toSender) {
		t.Fatal("sender was not told to dial the target")
	}
	if !bytes.Contains(targetConn.writeBuffer.Bytes(), toTarget) {
		t.Fatal("target was not told to dial the sender")
	}
}
