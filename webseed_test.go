package torrent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anacrolix/torrent/metainfo"
)

func TestWebseedSpansSingleAndMulti(t *testing.T) {
	length := int64(32)
	info := &metainfo.Info{PieceLength: 16, Length: length, Name: "movie.mkv"}
	tor := &Torrent{info: info, length: &length}
	tor.files = &[]*File{{path: "movie.mkv", offset: 0, length: 32}}
	spans, err := tor.webseedSpans("http://cdn.example/movie.mkv", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 1 || spans[0].rawURL != "http://cdn.example/movie.mkv" || spans[0].start != 16 || spans[0].n != 16 {
		t.Fatalf("%+v", spans)
	}

	info.Length = 0
	info.Files = []metainfo.FileInfo{{Length: 32}}
	tor.files = &[]*File{{path: "Show/ep.mkv", offset: 0, length: 32}}
	spans, err = tor.webseedSpans("http://cdn.example/pack/", 0)
	if err != nil {
		t.Fatal(err)
	}
	if spans[0].rawURL != "http://cdn.example/pack/Show/ep.mkv" || spans[0].start != 0 || spans[0].n != 16 {
		t.Fatalf("%+v", spans)
	}
}

func TestReadWebseedRange(t *testing.T) {
	var gotRange string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRange = r.Header.Get("Range")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("0123456789abcdef"))
	}))
	defer srv.Close()
	buf, err := readWebseedRange(t.Context(), srv.Client(), nil, srv.URL, "TorrServer", 0, 16)
	if err != nil {
		t.Fatal(err)
	}
	if gotRange != "bytes=0-15" || string(buf) != "0123456789abcdef" {
		t.Fatalf("range %q body %q", gotRange, buf)
	}
}
