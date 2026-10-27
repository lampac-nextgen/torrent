package peer_protocol

import (
	"bufio"
	"bytes"
	"testing"
)

func TestMarshalFastExtensionIndex(t *testing.T) {
	for _, typ := range []MessageType{Have, AllowedFast, Suggest} {
		msg := Message{Type: typ, Index: 42}
		b := msg.MustMarshalBinary()
		var got Message
		d := Decoder{R: bufio.NewReader(bytes.NewReader(b)), MaxLength: 1 << 20}
		if err := d.Decode(&got); err != nil {
			t.Fatalf("%v: %v", typ, err)
		}
		if got.Type != typ || got.Index != 42 {
			t.Fatalf("%v: got type %v index %d", typ, got.Type, got.Index)
		}
	}
}
