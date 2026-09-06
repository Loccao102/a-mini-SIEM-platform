package apikey

import (
	"encoding/hex"
	"testing"
)

func TestSHA256KnownVector(t *testing.T) {
	if got := hex.EncodeToString(hashSHA256([]byte("abc"))); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatal(got)
	}
	if hashKey("abc") != "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatal("missing hash version")
	}
}
