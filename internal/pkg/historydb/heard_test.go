package historydb

import "testing"

func TestHeardAddAndExpiry(t *testing.T) {
	h := NewHeardList()
	if h.Heard("N0CALL") {
		t.Fatal("empty list should not report heard")
	}
	h.Add("N0CALL-9")
	if !h.Heard("n0call-9") {
		t.Error("Heard should be case-insensitive")
	}
	if h.Heard("OTHER") {
		t.Error("unrelated station should not be heard")
	}
	if h.Len() != 1 {
		t.Errorf("Len = %d, want 1", h.Len())
	}
	// Empty callsigns are ignored.
	h.Add("")
	if h.Len() != 1 {
		t.Errorf("Len = %d after empty add, want 1", h.Len())
	}
}

func TestHeardBounded(t *testing.T) {
	h := NewHeardList()
	for i := 0; i < maxHeard+50; i++ {
		h.Add(string(rune('A'+i%26)) + itoa(i))
	}
	if h.Len() > maxHeard {
		t.Errorf("Len = %d exceeds cap %d", h.Len(), maxHeard)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
