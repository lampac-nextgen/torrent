package torrent

import (
	"encoding/json"
	"io"
	"sync"
	"testing"
)

// Peer writers bump Client.stats atomically while WriteStatus and JSON read them.
// Fails under -race if either read goes back to a plain load.
func TestClientStatsReadsAreAtomic(t *testing.T) {
	cl, err := NewClient(utpOnlyConfig(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				cl.stats.BytesWritten.Add(1)
			}
		}
	}()
	for range 50 {
		cl.WriteStatus(io.Discard)
		if _, err := json.Marshal(&cl.stats.BytesWritten); err != nil {
			t.Fatal(err)
		}
	}
	close(stop)
	wg.Wait()
}
