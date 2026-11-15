package torrent

import (
	"context"
	"errors"
	"io"
	"runtime"
	"testing"
	"time"
)

// newCloseTestReader builds a reader on a torrent with info but no data, so a Read waits for the
// swarm. The Client has no networking: only its lock and the event the readers wait on.
func newCloseTestReader(t *testing.T) (*Client, *reader) {
	t.Helper()
	cl := &Client{config: &ClientConfig{}}
	cl.event.L = cl.locker()
	tor := newTierTestTorrent(t, 1<<20, 4)
	tor.cl = cl
	tor.networkingEnabled = true
	tor.chunkSize = 16 << 10
	r := &reader{mu: cl.locker(), t: tor, length: *tor.length, readahead: 1 << 20}
	tor.readers[r] = struct{}{}
	return cl, r
}

func readResult(r *reader, ctx context.Context) <-chan error {
	done := make(chan error, 1)
	go func() {
		_, err := r.ReadContext(ctx, make([]byte, 1024))
		done <- err
	}()
	return done
}

func waitErr(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("Read is still blocked")
		return nil
	}
}

// TestReaderCloseWakesBlockedRead: Close used to only unregister the reader, so a Read waiting
// for data that never came stayed blocked until the torrent closed.
func TestReaderCloseWakesBlockedRead(t *testing.T) {
	_, r := newCloseTestReader(t)
	done := readResult(r, context.Background())
	// Give the Read time to reach event.Wait. Close before that is also fine: waitAvailable
	// checks the flag first.
	time.Sleep(50 * time.Millisecond)
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waitErr(t, done); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("got %v, want io.ErrClosedPipe", err)
	}
}

func TestReaderCloseTwice(t *testing.T) {
	_, r := newCloseTestReader(t)
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.t.readers[r]; ok {
		t.Fatal("closed reader is still registered")
	}
	if _, err := r.Read(make([]byte, 1)); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Read after Close: got %v, want io.ErrClosedPipe", err)
	}
}

// TestReadContextCancelStillWakes: the cancel watcher must keep working when the caller's
// context ends while the Read waits.
func TestReadContextCancelStillWakes(t *testing.T) {
	_, r := newCloseTestReader(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := readResult(r, ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()
	if err := waitErr(t, done); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

// TestReadContextWatcherSkipsClientLock: every ReadContext with a cancelable context starts a
// watcher. After a normal return it must exit without the exclusive Client lock; it used to queue
// for it, so a held lock pinned one goroutine per Read.
func TestReadContextWatcherSkipsClientLock(t *testing.T) {
	cl, r := newCloseTestReader(t)
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	base := runtime.NumGoroutine()
	for i := 0; i < 20; i++ {
		if _, err := r.ReadContext(ctx, make([]byte, 1)); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("got %v", err)
		}
	}
	cl.lock()
	defer cl.unlock()
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > base {
		if time.Now().After(deadline) {
			t.Fatalf("%d watcher goroutines wait for the Client lock", runtime.NumGoroutine()-base)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
