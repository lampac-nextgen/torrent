package torrent

import (
	"testing"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

type countingStorage struct{ opens int }

func (s *countingStorage) OpenTorrent(*metainfo.Info, metainfo.Hash) (storage.TorrentImpl, error) {
	s.opens++
	return completeTorrent{}, nil
}
func (*countingStorage) Close() error { return nil }

type completeTorrent struct{}

func (completeTorrent) Piece(metainfo.Piece) storage.PieceImpl { return completePiece{} }
func (completeTorrent) Close() error                           { return nil }

type completePiece struct{}

func (completePiece) ReadAt(b []byte, _ int64) (int, error)  { return len(b), nil }
func (completePiece) WriteAt(b []byte, _ int64) (int, error) { return len(b), nil }
func (completePiece) MarkComplete() error                    { return nil }
func (completePiece) MarkNotComplete() error                 { return nil }
func (completePiece) Completion() storage.Completion {
	return storage.Completion{Complete: true, Ok: true}
}

// AddTorrentSpec on a torrent that already has info must keep its storage,
// pieces and files: readers hold them without the client lock.
func TestAddTorrentSpecKeepsExistingInfo(t *testing.T) {
	store := &countingStorage{}
	cfg := utpOnlyConfig(t, 0)
	cfg.DefaultStorage = store
	cl, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	infoBytes, err := bencode.Marshal(metainfo.Info{
		Name:        "a",
		PieceLength: 16 << 10,
		Length:      16 << 10,
		Pieces:      make([]byte, 20),
	})
	if err != nil {
		t.Fatal(err)
	}
	spec := &TorrentSpec{InfoHash: metainfo.HashBytes(infoBytes), InfoBytes: infoBytes}

	tor, _, err := cl.AddTorrentSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	file := tor.Files()[0]

	again, isNew, err := cl.AddTorrentSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	if isNew || again != tor {
		t.Fatalf("re-add returned a new torrent")
	}
	if store.opens != 1 {
		t.Fatalf("storage opened %d times, want 1", store.opens)
	}
	if tor.Files()[0] != file {
		t.Fatal("files rebuilt on re-add")
	}
}
