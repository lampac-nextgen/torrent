package torrent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/anacrolix/missinggo/bitmap"
	"golang.org/x/time/rate"
)

type webseedSpan struct {
	rawURL string
	start  int64
	n      int64
}

func (t *Torrent) addWebseedURLs(urls []string) {
	if t.cl.config != nil && t.cl.config.DisableWebseeds {
		return
	}
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if t.webseedKnown == nil {
			t.webseedKnown = map[string]struct{}{}
		}
		if _, ok := t.webseedKnown[u]; ok {
			continue
		}
		t.webseedKnown[u] = struct{}{}
		t.webseedURLs = append(t.webseedURLs, u)
	}
	t.startWebseeds()
}

func (t *Torrent) startWebseeds() {
	if !t.haveInfo() || (t.cl.config != nil && t.cl.config.DisableWebseeds) {
		return
	}
	for _, u := range t.webseedURLs {
		if t.webseedStarted == nil {
			t.webseedStarted = map[string]struct{}{}
		}
		if _, ok := t.webseedStarted[u]; ok {
			continue
		}
		t.webseedStarted[u] = struct{}{}
		go t.runWebseed(u)
	}
}

func (t *Torrent) runWebseed(base string) {
	for {
		select {
		case <-t.closed.LockedChan(t.cl.locker()):
			return
		default:
		}
		piece, ok := t.claimWebseedPiece()
		if !ok {
			select {
			case <-t.closed.LockedChan(t.cl.locker()):
				return
			case <-time.After(500 * time.Millisecond):
			}
			continue
		}
		err := t.fetchWebseedPiece(base, piece)
		t.releaseWebseedPiece(piece)
		if err != nil {
			select {
			case <-t.closed.LockedChan(t.cl.locker()):
				return
			case <-time.After(2 * time.Second):
			}
		}
	}
}

func (t *Torrent) claimWebseedPiece() (pieceIndex, bool) {
	t.cl.lock()
	defer t.cl.unlock()
	if t.closed.IsSet() || !t.haveInfo() {
		return 0, false
	}
	var found pieceIndex
	ok := false
	consider := func(i int) bool {
		idx := pieceIndex(i)
		if _, busy := t.webseedBusy[idx]; busy || !t.wantPieceIndex(idx) {
			return true
		}
		found = idx
		ok = true
		return false
	}
	if !ok {
		t.readerNowPieces.IterTyped(consider)
	}
	if !ok {
		t.readerNextPieces.IterTyped(consider)
	}
	if !ok {
		t.readerReadaheadPieces.IterTyped(consider)
	}
	if !ok {
		t.readerZonePieces.IterTyped(consider)
	}
	if !ok {
		t.pendingPieces.IterTyped(func(i bitmap.BitIndex) bool {
			return consider(int(i))
		})
	}
	if !ok {
		return 0, false
	}
	if t.webseedBusy == nil {
		t.webseedBusy = map[pieceIndex]struct{}{}
	}
	t.webseedBusy[found] = struct{}{}
	return found, true
}

func (t *Torrent) releaseWebseedPiece(piece pieceIndex) {
	t.cl.lock()
	delete(t.webseedBusy, piece)
	t.cl.unlock()
}

// webseedSpans maps a piece onto HTTP ranges. Single-file url-list entries point at the file.
// Multi-file entries are a directory; the info name and path are appended.
func (t *Torrent) webseedSpans(base string, piece pieceIndex) ([]webseedSpan, error) {
	if t.info == nil || t.files == nil {
		return nil, fmt.Errorf("no info")
	}
	begin := int64(piece) * t.info.PieceLength
	end := begin + int64(t.pieceLength(piece))
	single := len(t.info.Files) == 0
	var spans []webseedSpan
	for _, f := range *t.files {
		fEnd := f.offset + f.length
		if fEnd <= begin || f.offset >= end || f.length == 0 {
			continue
		}
		lo := begin
		if f.offset > lo {
			lo = f.offset
		}
		hi := end
		if fEnd < hi {
			hi = fEnd
		}
		raw := base
		if !single {
			parts := strings.Split(f.path, "/")
			joined, err := url.JoinPath(base, parts...)
			if err != nil {
				return nil, err
			}
			raw = joined
		}
		spans = append(spans, webseedSpan{rawURL: raw, start: lo - f.offset, n: hi - lo})
	}
	if len(spans) == 0 {
		return nil, fmt.Errorf("piece %d has no file span", piece)
	}
	return spans, nil
}

func (cl *Client) webseedHTTP() *http.Client {
	cl.webseedOnce.Do(func() {
		tr := &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          8,
			IdleConnTimeout:       30 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
		}
		if cl.config != nil && cl.config.HTTPProxy != nil {
			tr.Proxy = cl.config.HTTPProxy
		}
		cl.webseedClient = &http.Client{Transport: tr}
	})
	return cl.webseedClient
}

func readWebseedRange(ctx context.Context, client *http.Client, lim *rate.Limiter, rawURL, userAgent string, start, n int64) ([]byte, error) {
	if n <= 0 {
		return nil, fmt.Errorf("empty range")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, start+n-1))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("webseed %s: %s", rawURL, resp.Status)
	}
	if resp.StatusCode == http.StatusOK && resp.ContentLength != n {
		return nil, fmt.Errorf("webseed %s ignored range", rawURL)
	}
	buf := make([]byte, n)
	got := 0
	for got < len(buf) {
		chunk := len(buf) - got
		if chunk > 16<<10 {
			chunk = 16 << 10
		}
		if lim != nil && lim.Limit() != rate.Inf && lim.Burst() >= chunk {
			if err := lim.WaitN(ctx, chunk); err != nil {
				return nil, err
			}
		}
		nn, err := io.ReadFull(resp.Body, buf[got:got+chunk])
		got += nn
		if err != nil {
			return nil, err
		}
	}
	return buf, nil
}

func (t *Torrent) fetchWebseedPiece(base string, piece pieceIndex) error {
	t.cl.lock()
	spans, err := t.webseedSpans(base, piece)
	httpc := t.cl.webseedHTTP()
	var lim *rate.Limiter
	if t.cl.config != nil {
		lim = t.cl.config.DownloadRateLimiter
	}
	ua := ""
	if t.cl.config != nil {
		ua = t.cl.config.HTTPUserAgent
	}
	t.cl.unlock()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var pieceOff int64
	for _, sp := range spans {
		data, err := readWebseedRange(ctx, httpc, lim, sp.rawURL, ua, sp.start, sp.n)
		if err != nil {
			return err
		}
		for len(data) > 0 {
			n := len(data)
			if n > 16<<10 {
				n = 16 << 10
			}
			if err := t.writeWebseed(piece, pieceOff, data[:n]); err != nil {
				return err
			}
			pieceOff += int64(n)
			data = data[n:]
		}
	}
	t.cl.lock()
	defer t.cl.unlock()
	if t.closed.IsSet() || t.pieceComplete(piece) {
		return nil
	}
	if pieceOff == int64(t.pieceLength(piece)) {
		t.queuePieceCheck(piece)
	}
	return nil
}

func (t *Torrent) writeWebseed(piece pieceIndex, begin int64, data []byte) error {
	t.cl.lock()
	defer t.cl.unlock()
	if t.closed.IsSet() || t.storage == nil || t.pieceComplete(piece) {
		return nil
	}
	if !t.pieceWanted(piece) && !t.wantPieceIndex(piece) {
		return fmt.Errorf("piece %d no longer wanted", piece)
	}
	return t.writeChunk(int(piece), begin, data)
}
