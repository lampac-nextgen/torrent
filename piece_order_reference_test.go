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
	for i := range pieces {
		if rnd.Intn(4) != 0 {
			cn.peerPieces.Add(i)
		}
		switch rnd.Intn(10) {
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
	return cn, classes
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
	for run := range 500 {
		cn, c := randomPieceOrderFixture(t, rnd, 1+rnd.Intn(300))
		limit := 1 + rnd.Intn(400)
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
