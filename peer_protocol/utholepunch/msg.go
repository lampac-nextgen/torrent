// Package utholepunch is the BEP 55 ut_holepunch payload.
package utholepunch

import (
	"encoding/binary"
	"fmt"
	"net/netip"
)

const ExtensionName = "ut_holepunch"

type (
	Msg struct {
		MsgType  MsgType
		AddrPort netip.AddrPort
		ErrCode  ErrCode
	}
	MsgType  byte
	AddrType byte
)

const (
	Rendezvous MsgType = iota
	Connect
	Error
)

const (
	Ipv4 AddrType = iota
	Ipv6
)

func (m *Msg) UnmarshalBinary(b []byte) error {
	if len(b) < 12 {
		return fmt.Errorf("buffer too small")
	}
	m.MsgType = MsgType(b[0])
	b = b[1:]
	addrType := AddrType(b[0])
	b = b[1:]
	var addr netip.Addr
	switch addrType {
	case Ipv4:
		if len(b) < 4+2+4 {
			return fmt.Errorf("short ipv4 holepunch message")
		}
		addr = netip.AddrFrom4([4]byte(b[:4]))
		b = b[4:]
	case Ipv6:
		if len(b) < 16+2+4 {
			return fmt.Errorf("short ipv6 holepunch message")
		}
		addr = netip.AddrFrom16([16]byte(b[:16]))
		b = b[16:]
	default:
		return fmt.Errorf("unhandled addr type %d", addrType)
	}
	port := binary.BigEndian.Uint16(b)
	b = b[2:]
	m.AddrPort = netip.AddrPortFrom(addr, port)
	m.ErrCode = ErrCode(binary.BigEndian.Uint32(b))
	b = b[4:]
	if len(b) != 0 {
		return fmt.Errorf("%d trailing bytes", len(b))
	}
	return nil
}

func (m *Msg) MarshalBinary() ([]byte, error) {
	addr := m.AddrPort.Addr().Unmap()
	buf := make([]byte, 0, 24)
	buf = append(buf, byte(m.MsgType))
	switch {
	case addr.Is4():
		buf = append(buf, byte(Ipv4))
		a := addr.As4()
		buf = append(buf, a[:]...)
	case addr.Is6():
		buf = append(buf, byte(Ipv6))
		a := addr.As16()
		buf = append(buf, a[:]...)
	default:
		return nil, fmt.Errorf("unhandled addr %v", addr)
	}
	buf = binary.BigEndian.AppendUint16(buf, m.AddrPort.Port())
	buf = binary.BigEndian.AppendUint32(buf, uint32(m.ErrCode))
	return buf, nil
}
