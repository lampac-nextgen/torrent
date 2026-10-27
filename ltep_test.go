package torrent

import (
	"bytes"
	"sync"
	"testing"

	"github.com/anacrolix/missinggo/bitmap"
	pp "github.com/anacrolix/torrent/peer_protocol"
)

func TestDontHaveClearsPiece(t *testing.T) {
	var mu sync.Mutex
	cn := &connection{
		peerSentHaveAll: true,
		PeerExtensionIDs: map[pp.ExtensionName]pp.ExtensionNumber{
			pp.ExtensionNameDontHave: dontHaveExtendedId,
		},
	}
	cn.writerCond.L = &mu
	cn.peerPieces.Add(3)
	if err := cn.onDontHave([]byte{0, 0, 0, 3}); err != nil {
		t.Fatal(err)
	}
	if cn.PeerHasPiece(3) {
		t.Fatal("still has piece 3")
	}
	if !cn.PeerHasPiece(1) {
		t.Fatal("have-all should still cover other pieces")
	}
}

func TestUploadOnlyStopsUnchoke(t *testing.T) {
	cn := chokeFixture(1)
	defer stopChokeFixture(cn)
	cn.t.cl.config.NoUpload = false
	cn.PeerInterested = false
	cn.uploadOnly = true
	if cn.uploadAllowed() {
		t.Fatal("upload-only peer should not be an upload target")
	}
}

func TestSuggestIgnoredOutsideReader(t *testing.T) {
	cn := chokeFixture(2)
	defer stopChokeFixture(cn)
	cn.suggestOK = false
	cn.noteSuggest(0)
	if cn.suggestOK {
		t.Fatal("suggest outside the reader window was kept")
	}
}

func TestDontHaveNotRequestedAfterHaveAll(t *testing.T) {
	cn := chokeFixture(2)
	defer stopChokeFixture(cn)
	var mu sync.Mutex
	cn.writerCond.L = &mu
	for i := 0; i < 2; i++ {
		cn.t.pendingPieces.Set(i, PiecePriorityNormal.BitmapPriority())
	}
	cn.PeerChoked = false
	if err := cn.onDontHave([]byte{0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	cn.fillWriteBuffer(func(msg pp.Message) bool {
		if msg.Type == pp.Request && msg.Index == 0 {
			t.Fatal("requested a piece the peer does not have")
		}
		return true
	})
	if len(cn.requests) == 0 {
		t.Fatal("stopped requesting pieces the peer still has")
	}
}

func TestHaveClearsDontHave(t *testing.T) {
	cn := chokeFixture(1)
	defer stopChokeFixture(cn)
	var mu sync.Mutex
	cn.writerCond.L = &mu
	cn.t.pendingPieces.Set(0, PiecePriorityNormal.BitmapPriority())
	if err := cn.onDontHave([]byte{0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if cn.PeerHasPiece(0) {
		t.Fatal("dont-have ignored")
	}
	if err := cn.peerSentHave(0); err != nil {
		t.Fatal(err)
	}
	if !cn.PeerHasPiece(0) {
		t.Fatal("have did not restore the piece")
	}
}

func TestSentDontHavePostsExtended(t *testing.T) {
	var mu sync.Mutex
	cn := &connection{
		writeBuffer: new(bytes.Buffer),
		PeerExtensionIDs: map[pp.ExtensionName]pp.ExtensionNumber{
			pp.ExtensionNameDontHave: dontHaveExtendedId,
		},
	}
	cn.writerCond.L = &mu
	cn.sentHaves.Add(bitmap.BitIndex(4))
	cn.t = &Torrent{cl: &Client{config: &ClientConfig{}}}
	cn.sentDontHave(4)
	if cn.sentHaves.Get(bitmap.BitIndex(4)) {
		t.Fatal("have bit still set")
	}
	if cn.writeBuffer.Len() == 0 {
		t.Fatal("no message posted")
	}
}
