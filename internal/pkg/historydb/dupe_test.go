package historydb

import (
	"testing"
	"time"
)

func TestDupeChecker(t *testing.T) {
	d := NewDupeChecker(50 * time.Millisecond)

	if d.Seen("A>B:>hi") {
		t.Error("first sighting should not be a duplicate")
	}
	if !d.Seen("A>B:>hi") {
		t.Error("immediate repeat should be a duplicate")
	}
	if d.Seen("A>B:>other") {
		t.Error("different packet should not be a duplicate")
	}

	// After the window expires, the same packet is no longer a duplicate.
	time.Sleep(70 * time.Millisecond)
	if d.Seen("A>B:>hi") {
		t.Error("repeat after window should not be a duplicate")
	}
}

// The via path (and appended q-construct) must be ignored: the same source and
// information field relayed through different routes is one packet.
func TestDupeCheckerIgnoresPath(t *testing.T) {
	d := NewDupeChecker(time.Minute)

	if d.Seen("CALL>APRS,WIDE1-1:>status") {
		t.Error("first sighting should not be a duplicate")
	}
	if !d.Seen("CALL>APRS,qAO,IGATE:>status") {
		t.Error("same source+info via a different path should be a duplicate")
	}
	if !d.Seen("CALL>APRS,DIGI1*,DIGI2*,qAR,X:>status") {
		t.Error("same source+info via yet another path should be a duplicate")
	}

	// A different source is not a duplicate even with the same info field.
	if d.Seen("OTHER>APRS:>status") {
		t.Error("different source should not be a duplicate")
	}
	// A different info field is not a duplicate.
	if d.Seen("CALL>APRS:>different") {
		t.Error("different info field should not be a duplicate")
	}
}

// Lightly corrupted copies (trailing spaces, high-bit, low and DEL bytes)
// of an already-seen packet must be recognised as duplicates.
func TestDupeCheckerVariants(t *testing.T) {
	cases := []struct {
		name  string
		first string
		dupe  string
	}{
		{"trailing space", "C>AP:>hello", "C>AP:>hello  "},
		{"trailing space reversed", "C>AP:>hello   ", "C>AP:>hello"},
		{"high bit cleared", "C>AP:>caf\xe9", "C>AP:>caf\x69"},
		{"high bit to space", "C>AP:>caf\xe9", "C>AP:>caf "},
		{"low byte stripped", "C>AP:>ab\x01cd", "C>AP:>abcd"},
		{"low byte to space", "C>AP:>ab\x01cd", "C>AP:>ab cd"},
		{"del stripped", "C>AP:>ab\x7fcd", "C>AP:>abcd"},
		{"del to space", "C>AP:>ab\x7fcd", "C>AP:>ab cd"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDupeChecker(time.Minute)
			if d.Seen(tc.first) {
				t.Fatalf("first sighting %q should not be a duplicate", tc.first)
			}
			if !d.Seen(tc.dupe) {
				t.Errorf("corrupted copy %q of %q should be a duplicate", tc.dupe, tc.first)
			}
		})
	}
}

// A packet with no information field separator falls back to whole-line
// matching and still suppresses exact repeats.
func TestDupeCheckerNoInfoField(t *testing.T) {
	d := NewDupeChecker(time.Minute)
	if d.Seen("garbage-no-colon") {
		t.Error("first sighting should not be a duplicate")
	}
	if !d.Seen("garbage-no-colon") {
		t.Error("exact repeat should be a duplicate")
	}
}
