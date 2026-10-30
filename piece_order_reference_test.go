package torrent

import (
	"math/rand"
	"slices"
	"testing"

	"github.com/anacrolix/missinggo/bitmap"
	"github.com/anacrolix/missinggo/iter"
)

// iterPieceOrderBeforeSkipTests is the scheduler order before iterPieceOrder, kept as a test-only
// oracle: it built a skip bitmap with Flip(peerPieces) and a Sub per priority class.
func (cn *connection) iterPieceOrderBeforeSkipTests(f func(pieceIndex) bool, now, next, readahead, zone bitmap.Bitmap) bool {
	var skip bitmap.Bitmap
	if !cn.peerSentHaveAll {
		skip = bitmap.Flip(cn.peerPieces, 0, bitmap.BitIndex(cn.t.numPieces()))
	}
	skip.Union(cn.t.completedPieces)
	skip.Union(cn.t.piecesQueuedForHash)
	distinct := func(cb iter.Callback) {
		for _, bm := range []bitmap.Bitmap{now, next, readahead, zone} {
			if !iter.All(func(i interface{}) bool {
				skip.Add(i.(int))
				return cb(i)
			}, bitmap.Sub(bm, skip).Iter) {
				return
			}
		}
	}
	return iter.All(
		func(_piece interface{}) bool {
			i := _piece.(bitmap.BitIndex)
			if cn.t.hashingPiece(i) {
				return true
			}
			return f(i)
		},
		distinct,
		func(cb iter.Callback) {
			cn.t.pendingPieces.IterTyped(func(piece int) bool {
				if skip.Contains(piece) {
					return true
				}
				more := cb(piece)
				skip.Add(piece)
				return more
			})
		},
	)
}

func randomPieceOrderFixture(t testing.TB, rnd *rand.Rand, pieces int) (cn *connection, classes [4]bitmap.Bitmap) {
	cn = chokeFixture(pieces)
	t.Cleanup(func() { stopChokeFixture(cn) })
	cn.peerSentHaveAll = rnd.Intn(4) == 0
	// A dense fixture puts the completed and queued sets above 4096 bits per chunk.
	spread := 10
	if rnd.Intn(2) == 0 {
		spread = 3
	}
	for i := range pieces {
		if rnd.Intn(4) != 0 {
			cn.peerPieces.Add(i)
		}
		switch rnd.Intn(spread) {
		case 0:
			cn.t.completedPieces.Add(i)
		case 1:
			cn.t.piecesQueuedForHash.Add(i)
		case 2:
			cn.t.pieces[i].hashing = true
		}
		if rnd.Intn(3) == 0 {
			cn.t.pendingPieces.Remove(i)
		}
	}
	for c := range classes {
		begin := rnd.Intn(pieces)
		end := begin + 1 + rnd.Intn(pieces/4+1)
		if end > pieces {
			end = pieces
		}
		classes[c].AddRange(begin, end)
	}
	// Above 4096 set bits roaring holds a chunk as a bitmap container and after RunOptimize as
	// runs; the dense skip buffer is filled differently for each kind.
	if rnd.Intn(2) == 0 {
		cn.peerPieces.AddRange(0, rnd.Intn(pieces)+1)
		begin := rnd.Intn(pieces)
		cn.t.completedPieces.AddRange(begin, begin+rnd.Intn(pieces-begin)+1)
		begin = rnd.Intn(pieces)
		cn.t.piecesQueuedForHash.AddRange(begin, begin+rnd.Intn(pieces-begin)+1)
		for _, bm := range []*bitmap.Bitmap{&cn.peerPieces, &cn.t.completedPieces, &cn.t.piecesQueuedForHash} {
			if bm.RB != nil {
				bm.RB.RunOptimize()
			}
		}
	}
	return cn, classes
}

func randomPieceCount(rnd *rand.Rand, run int) int {
	if run%5 == 0 {
		return 5000 + rnd.Intn(20000)
	}
	return 1 + rnd.Intn(300)
}

func collectOrder(iterate func(func(pieceIndex) bool) bool, limit int) (got []pieceIndex, all bool) {
	all = iterate(func(i pieceIndex) bool {
		got = append(got, i)
		return len(got) < limit
	})
	return got, all
}

// TestIterPieceOrderMatchesOracle checks that iterPieceOrder emits the same pieces in the same order,
// and stops at the same place, as the bitmap-building version it replaced.
func TestIterPieceOrderMatchesOracle(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	// Container kinds seen per set: peer, completed, queued. WriteDenseTo fills each kind
	// differently, and a bitmap container overwrites instead of OR-ing.
	var bitmapContainers, runContainers [3]uint64
	defer func() {
		for i := range 3 {
			if !t.Failed() && (bitmapContainers[i] == 0 || runContainers[i] == 0) {
				t.Errorf("set %d: fixtures built %d bitmap and %d run containers, want both", i, bitmapContainers[i], runContainers[i])
			}
		}
	}()
	for run := range 500 {
		pieces := randomPieceCount(rnd, run)
		cn, c := randomPieceOrderFixture(t, rnd, pieces)
		for i, bm := range []*bitmap.Bitmap{&cn.peerPieces, &cn.t.completedPieces, &cn.t.piecesQueuedForHash} {
			if bm.RB != nil {
				st := bm.RB.Stats()
				bitmapContainers[i] += st.BitmapContainers
				runContainers[i] += st.RunContainers
			}
		}
		limit := 1 + rnd.Intn(400)
		if run%3 == 0 {
			limit = pieces + 1
		}
		want, wantAll := collectOrder(func(f func(pieceIndex) bool) bool {
			return cn.iterPieceOrderBeforeSkipTests(f, c[0], c[1], c[2], c[3])
		}, limit)
		got, gotAll := collectOrder(func(f func(pieceIndex) bool) bool {
			return cn.iterPieceOrder(f, c[0], c[1], c[2], c[3])
		}, limit)
		if !slices.Equal(got, want) || gotAll != wantAll {
			t.Fatalf("run %d: order %v (all %v), oracle %v (all %v)", run, got, gotAll, want, wantAll)
		}
	}
}

func BenchmarkIterPieceOrder(b *testing.B) {
	cn, c := randomPieceOrderFixture(b, rand.New(rand.NewSource(1)), 4000)
	cn.peerSentHaveAll = false
	for name, iterate := range map[string]func(func(pieceIndex) bool) bool{
		"oracle": func(f func(pieceIndex) bool) bool { return cn.iterPieceOrderBeforeSkipTests(f, c[0], c[1], c[2], c[3]) },
		"skip-per-piece": func(f func(pieceIndex) bool) bool {
			return cn.iterPieceOrder(f, c[0], c[1], c[2], c[3])
		},
	} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				n := 0
				iterate(func(pieceIndex) bool { n++; return n < 64 })
			}
		})
	}
}
