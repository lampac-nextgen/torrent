package utholepunch

import (
	"net/netip"
	"testing"
)

func TestMsgRoundTrip(t *testing.T) {
	orig := Msg{
		MsgType:  Connect,
		AddrPort: netip.AddrPortFrom(netip.AddrFrom4([4]byte{203, 0, 113, 5}), 6881),
		ErrCode:  NotConnected,
	}
	b, err := orig.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var got Msg
	if err := got.UnmarshalBinary(b); err != nil {
		t.Fatal(err)
	}
	if got != orig {
		t.Fatalf("got %+v", got)
	}
}

func TestMsgRejectsShort(t *testing.T) {
	var m Msg
	if err := m.UnmarshalBinary([]byte{0, 0, 1, 2, 3}); err == nil {
		t.Fatal("expected error")
	}
}
