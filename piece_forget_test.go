package torrent

import (
	"testing"

	"github.com/anacrolix/log"
	"github.com/anacrolix/torrent/metainfo"
	pp "github.com/anacrolix/torrent/peer_protocol"
	"github.com/anacrolix/torrent/storage"
)

type emptyStore struct{}

func (emptyStore) Piece(metainfo.Piece) storage.PieceImpl { return emptyPiece{} }
func (emptyStore) Close() error                           { return nil }

// emptyPiece is storage that evicted the bytes: nothing is complete.
type emptyPiece struct{}

func (emptyPiece) ReadAt([]byte, int64) (int, error)      { return 0, nil }
func (emptyPiece) WriteAt(b []byte, _ int64) (int, error) { return len(b), nil }
func (emptyPiece) MarkComplete() error                    { return nil }
func (emptyPiece) MarkNotComplete() error                 { return nil }
func (emptyPiece) Completion() storage.Completion {
	return storage.Completion{Ok: true}
}

func forgetFixture(t *testing.T) *connection {
	t.Helper()
	cn := chokeFixture(2)
	t.Cleanup(func() { stopChokeFixture(cn) })
	cn.PeerChoked = false
	cn.t.storage = &storage.Torrent{TorrentImpl: emptyStore{}}
	cn.t.logger = log.Default
	p := &cn.t.pieces[0]
	p.storageCompletionOk = true
	p.dirtyChunks.Add(0)
	p.dirtyChunks.Add(1)
	p.dirtiers = map[*connection]struct{}{cn: {}}
	cn.peerTouchedPieces = map[pieceIndex]struct{}{0: {}}
	return cn
}

func requestedChunk0(cn *connection) bool {
	asked := false
	cn.fillWriteBuffer(func(m pp.Message) bool {
		if m.Type == pp.Request && m.Index == 0 && m.Begin == 0 {
			asked = true
		}
		return true
	})
	return asked
}

// TestPieceForgetRerequestsChunks checks that chunks of an evicted partial piece are fetched again
// and the peers that wrote them are not blamed for a later hash failure.
func TestPieceForgetRerequestsChunks(t *testing.T) {
	cn := forgetFixture(t)
	p := &cn.t.pieces[0]
	p.Forget()
	if n := p.dirtyChunks.Len(); n != 0 {
		t.Fatalf("%d chunks still dirty after Forget", n)
	}
	if p.dirtiers != nil || len(cn.peerTouchedPieces) != 0 {
		t.Fatal("Forget kept the peers that touched the piece")
	}
	if !requestedChunk0(cn) {
		t.Fatal("forgotten chunk was not requested again")
	}
}

// TestPieceForgetSkipsHashing checks that a piece under a hash keeps its dirty chunks.
func TestPieceForgetSkipsHashing(t *testing.T) {
	cn := forgetFixture(t)
	p := &cn.t.pieces[0]
	p.hashing = true
	p.Forget()
	if n := p.dirtyChunks.Len(); n != 2 {
		t.Fatalf("Forget cleared %d dirty chunks of a piece under hash", 2-n)
	}
	if requestedChunk0(cn) {
		t.Fatal("dirty chunk of a piece under hash was requested again")
	}
}
