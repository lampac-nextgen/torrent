package metainfo

import "testing"

func TestParseMagnetUriPaddedBase32ReturnsError(t *testing.T) {
	// 32 chars including '=' padding decodes to 18 bytes, not 20. Must not panic.
	_, err := ParseMagnetUri("magnet:?xt=urn:btih:MFRGGZDFMZTWQ2LKNNWG23TPOBYXE===")
	if err == nil {
		t.Fatal("want error for padded/truncated base32 btih")
	}
}

func TestParseMagnetUriBase32OK(t *testing.T) {
	m, err := ParseMagnetUri("magnet:?xt=urn:btih:MFRGGZDFMZTWQ2LKNNWG23TPOBYXE43U")
	if err != nil {
		t.Fatal(err)
	}
	if got := m.InfoHash.HexString(); len(got) != 40 {
		t.Fatalf("hex length %d, want 40: %s", len(got), got)
	}
}
