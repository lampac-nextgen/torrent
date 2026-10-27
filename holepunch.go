package torrent

import (
	"net"
	"net/netip"

	"github.com/anacrolix/missinggo"
	pp "github.com/anacrolix/torrent/peer_protocol"
	"github.com/anacrolix/torrent/peer_protocol/utholepunch"
)

// holepunchRelayCap bounds how many connected peers we ask to relay one failed dial.
// ponytail: a handful of relays is enough for NAT; asking the whole swarm is an announce storm.
const holepunchRelayCap = 8

func (c *connection) supportsHolepunch() bool {
	id, ok := c.PeerExtensionIDs[pp.ExtensionNameHolepunch]
	return ok && id != 0
}

// holepunchDialAddr is the address the other side should dial. Incoming peers
// are reached on the listen port from the extended handshake, not the ephemeral source port.
func (c *connection) holepunchDialAddr() IpPort {
	addr := c.remoteAddr
	if !c.outgoing && c.PeerListenPort > 0 && c.PeerListenPort <= 65535 {
		addr.Port = uint16(c.PeerListenPort)
	}
	return addr
}

func netipFromIPPort(ip net.IP, port uint16) (netip.AddrPort, bool) {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.AddrPort{}, false
	}
	return netip.AddrPortFrom(addr.Unmap(), port), true
}

func ipPortFromNetip(a netip.AddrPort) IpPort {
	return IpPort{IP: a.Addr().Unmap().AsSlice(), Port: a.Port()}
}

func (cl *Client) listensOn(addr IpPort) bool {
	var yes bool
	cl.eachListener(func(l socket) bool {
		if missinggo.AddrPort(l.Addr()) == int(addr.Port) && missinggo.AddrIP(l.Addr()).Equal(addr.IP) {
			yes = true
			return false
		}
		return true
	})
	return yes
}

func (t *Torrent) sendUtHolepunch(c *connection, mt utholepunch.MsgType, addr netip.AddrPort, code utholepunch.ErrCode) {
	id, ok := c.PeerExtensionIDs[pp.ExtensionNameHolepunch]
	if !ok || id == 0 || c.writeBuffer == nil {
		return
	}
	payload, err := (&utholepunch.Msg{MsgType: mt, AddrPort: addr, ErrCode: code}).MarshalBinary()
	if err != nil {
		return
	}
	c.Post(pp.Message{
		Type:            pp.Extended,
		ExtendedID:      id,
		ExtendedPayload: payload,
	})
}

// trySendHolepunchRendezvous asks connected peers to introduce addr after a dial failed.
// Caller holds t.cl's lock.
func (t *Torrent) trySendHolepunchRendezvous(addr IpPort) {
	target, ok := netipFromIPPort(addr.IP, addr.Port)
	if !ok {
		return
	}
	sent := 0
	for c := range t.conns {
		if c.closed.IsSet() || !c.supportsHolepunch() {
			continue
		}
		if c.holepunchDialAddr().String() == addr.String() {
			continue
		}
		t.sendUtHolepunch(c, utholepunch.Rendezvous, target, 0)
		sent++
		if sent >= holepunchRelayCap {
			break
		}
	}
}

func (t *Torrent) connsForHolepunch(addr netip.AddrPort) []*connection {
	want := ipPortFromNetip(addr)
	var ret []*connection
	for c := range t.conns {
		if c.closed.IsSet() {
			continue
		}
		if c.holepunchDialAddr().String() == want.String() || c.remoteAddr.String() == want.String() {
			ret = append(ret, c)
		}
	}
	return ret
}

// handleUtHolepunch implements the relay and the connect reaction. Caller holds the client lock.
func (t *Torrent) handleUtHolepunch(msg utholepunch.Msg, sender *connection) error {
	if !sender.supportsHolepunch() {
		return nil
	}
	switch msg.MsgType {
	case utholepunch.Rendezvous:
		target := ipPortFromNetip(msg.AddrPort)
		if t.cl.listensOn(target) || sender.holepunchDialAddr().String() == target.String() {
			t.sendUtHolepunch(sender, utholepunch.Error, msg.AddrPort, utholepunch.NoSelf)
			return nil
		}
		peers := t.connsForHolepunch(msg.AddrPort)
		if len(peers) == 0 {
			t.sendUtHolepunch(sender, utholepunch.Error, msg.AddrPort, utholepunch.NotConnected)
			return nil
		}
		senderAddr, ok := netipFromIPPort(sender.holepunchDialAddr().IP, sender.holepunchDialAddr().Port)
		if !ok {
			t.sendUtHolepunch(sender, utholepunch.Error, msg.AddrPort, utholepunch.NoSuchPeer)
			return nil
		}
		for _, pc := range peers {
			if !pc.supportsHolepunch() {
				t.sendUtHolepunch(sender, utholepunch.Error, msg.AddrPort, utholepunch.NoSupport)
				continue
			}
			t.sendUtHolepunch(sender, utholepunch.Connect, msg.AddrPort, 0)
			t.sendUtHolepunch(pc, utholepunch.Connect, senderAddr, 0)
		}
		return nil
	case utholepunch.Connect:
		addr := ipPortFromNetip(msg.AddrPort)
		if t.addrActive(addr.String()) {
			return nil
		}
		t.initiateConn(Peer{
			IP:     append(net.IP(nil), addr.IP...),
			Port:   int(addr.Port),
			Source: peerSourceHolepunch,
		})
		return nil
	case utholepunch.Error:
		return nil
	default:
		return nil
	}
}
