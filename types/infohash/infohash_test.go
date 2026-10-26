package infohash

import "testing"

func TestFromHexStringBadLength(t *testing.T) {
	var h T
	err := h.FromHexString("MFRGGZDFMZTWQ2LKNNWG23TPOBYXE43U")
	if err == nil {
		t.Fatal("want error for 32-char base32")
	}
}

func TestFromHexStringPanicsOnlyViaConstructor(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("FromHexString (constructor) must still panic on bad input")
		}
	}()
	_ = FromHexString("short")
}
