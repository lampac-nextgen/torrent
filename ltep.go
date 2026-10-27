package torrent

import (
	"encoding/binary"

	"github.com/anacrolix/missinggo/bitmap"
	pp "github.com/anacrolix/torrent/peer_protocol"
)

func (c *connection) onUploadOnly(payload []byte) error {
	if len(payload) < 1 {
		return nil
	}
	c.uploadOnly = payload[0] != 0
	return nil
}

func (c *connection) onDontHave(payload []byte) error {
	if len(payload) < 4 {
		return nil
	}
	idx := int(binary.BigEndian.Uint32(payload))
	c.peerDontHave.Add(idx)
	c.peerPieces.Remove(idx)
	c.updateRequests()
	return nil
}

func (c *connection) maybeSendUploadOnly() {
	id, ok := c.PeerExtensionIDs[pp.ExtensionNameUploadOnly]
	if !ok || id == 0 {
		return
	}
	v := byte(0)
	if c.t.haveAllPieces() || c.t.cl.config.NoUpload {
		v = 1
	}
	c.Post(pp.Message{
		Type:            pp.Extended,
		ExtendedID:      id,
		ExtendedPayload: []byte{v},
	})
}

func (c *connection) sentDontHave(piece pieceIndex) {
	if !c.sentHaves.Get(bitmap.BitIndex(piece)) {
		return
	}
	id, ok := c.PeerExtensionIDs[pp.ExtensionNameDontHave]
	if !ok || id == 0 {
		return
	}
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(piece))
	c.Post(pp.Message{
		Type:            pp.Extended,
		ExtendedID:      id,
		ExtendedPayload: buf[:],
	})
	c.sentHaves.Remove(bitmap.BitIndex(piece))
}
