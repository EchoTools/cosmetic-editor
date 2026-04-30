package data

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// buildMinimalCosmeticListBytes builds a minimal valid cosmetic list binary
// with the given ListCount and a correctly-sized slice of zero-filled entries.
func buildMinimalCosmeticListBytes(listCount uint64) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, [8]byte{})             // padding
	binary.Write(&buf, binary.LittleEndian, uint64(0))             // ListSize
	binary.Write(&buf, binary.LittleEndian, [12]byte{})            // padding
	binary.Write(&buf, binary.LittleEndian, uint32(0))             // Unk1
	binary.Write(&buf, binary.LittleEndian, [8]byte{})             // padding
	binary.Write(&buf, binary.LittleEndian, listCount)             // ListCount
	binary.Write(&buf, binary.LittleEndian, listCount)             // ListCount2
	for i := uint64(0); i < listCount; i++ {
		binary.Write(&buf, binary.LittleEndian, [664]byte{})
	}
	return buf.Bytes()
}

// TestBytesToCosmeticList_TooSmall verifies an error is returned for files smaller
// than the minimum 56-byte header (bug: previously would panic or silently fail).
func TestBytesToCosmeticList_TooSmall(t *testing.T) {
	_, err := BytesToCosmeticList([]byte{0x00, 0x01})
	if err == nil {
		t.Fatal("expected error for input shorter than header, got nil")
	}
}

// TestBytesToCosmeticList_BinaryReadError verifies that a truncated entry causes
// an error to be returned rather than silently yielding a partial result.
// This is the direct TDD test for the binary.Read error-check fix.
func TestBytesToCosmeticList_BinaryReadError(t *testing.T) {
	// Build a valid header claiming 1 entry, but truncate the entry data.
	b := buildMinimalCosmeticListBytes(1)
	truncated := b[:56+100] // header OK, but entry is only 100 of the required 664 bytes
	_, err := BytesToCosmeticList(truncated)
	if err == nil {
		t.Fatal("expected error for truncated entry, got nil")
	}
}

// TestBytesToCosmeticList_ExceedsMaxEntries verifies the overflow-guard cap is
// enforced. A crafted header claiming more than maxCosmeticEntries entries must
// return an error immediately rather than allocating gigabytes of memory.
func TestBytesToCosmeticList_ExceedsMaxEntries(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, [8]byte{})
	binary.Write(&buf, binary.LittleEndian, uint64(0))
	binary.Write(&buf, binary.LittleEndian, [12]byte{})
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	binary.Write(&buf, binary.LittleEndian, [8]byte{})
	binary.Write(&buf, binary.LittleEndian, uint64(maxCosmeticEntries+1)) // over the cap
	binary.Write(&buf, binary.LittleEndian, uint64(0))
	b := buf.Bytes()

	_, err := BytesToCosmeticList(b)
	if err == nil {
		t.Fatalf("expected error for ListCount > %d, got nil", maxCosmeticEntries)
	}
}

// TestBytesToCosmeticList_RoundTrip verifies that encoding and then decoding a
// list with zero entries produces the same structure without errors.
func TestBytesToCosmeticList_RoundTrip(t *testing.T) {
	original := CosmeticList{
		ListSize:  0,
		Unk1:      0,
		ListCount: 0,
		ListCount2: 0,
	}
	b, err := CosmeticListToBytes(original)
	if err != nil {
		t.Fatalf("CosmeticListToBytes: %v", err)
	}
	decoded, err := BytesToCosmeticList(b)
	if err != nil {
		t.Fatalf("BytesToCosmeticList: %v", err)
	}
	if decoded.ListCount != 0 {
		t.Errorf("expected ListCount 0, got %d", decoded.ListCount)
	}
}

// TestBytesToCosmeticList_ValidOneEntry confirms a well-formed single-entry file
// parses successfully, exercising the binary.Read happy path.
func TestBytesToCosmeticList_ValidOneEntry(t *testing.T) {
	b := buildMinimalCosmeticListBytes(1)
	got, err := BytesToCosmeticList(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.CosmeticEntries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(got.CosmeticEntries))
	}
}
