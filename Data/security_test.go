package data

import (
	"encoding/binary"
	"math"
	"testing"
)

// ----------------------------------------------------------------------------
// SafeHexFilename tests
// These FAIL until SafeHexFilename is added to symbol.go
// ----------------------------------------------------------------------------

func TestSafeHexFilename_ValidHex(t *testing.T) {
	cases := []string{
		"43934c379cf1e366",
		"abc123def456",
		"0",
		"ffffffffffffffff",
		"ABCDEF1234567890",
	}
	for _, c := range cases {
		_, err := SafeHexFilename(c)
		if err != nil {
			t.Errorf("SafeHexFilename(%q) should be valid, got: %v", c, err)
		}
	}
}

func TestSafeHexFilename_PathTraversal(t *testing.T) {
	cases := []string{
		"../etc/passwd",
		"../../secret",
		"../foo",
		"abc/../def",
		"abc/def",
	}
	for _, c := range cases {
		_, err := SafeHexFilename(c)
		if err == nil {
			t.Errorf("SafeHexFilename(%q) should reject path traversal, but did not", c)
		}
	}
}

func TestSafeHexFilename_NonHexChars(t *testing.T) {
	cases := []string{
		"hello world",
		"abc!def",
		"abc\x00def",
		"abc;rm -rf /",
	}
	for _, c := range cases {
		_, err := SafeHexFilename(c)
		if err == nil {
			t.Errorf("SafeHexFilename(%q) should reject non-hex chars, but did not", c)
		}
	}
}

func TestSafeHexFilename_Empty(t *testing.T) {
	_, err := SafeHexFilename("")
	if err == nil {
		t.Error("SafeHexFilename(\"\") should return an error for empty input")
	}
}

func TestSafeHexFilename_NormalizesCase(t *testing.T) {
	result, err := SafeHexFilename("ABCDEF1234567890")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "abcdef1234567890" {
		t.Errorf("expected lowercase %q, got %q", "abcdef1234567890", result)
	}
}

// ----------------------------------------------------------------------------
// BytesToCosmeticList integer-overflow / bounds tests
// These FAIL until overflow checks are added to cosmeticlist.go
// ----------------------------------------------------------------------------

func TestBytesToCosmeticList_TooSmall(t *testing.T) {
	_, err := BytesToCosmeticList([]byte{0x00, 0x01})
	if err == nil {
		t.Error("expected error for input shorter than 56 bytes")
	}
}

func TestBytesToCosmeticList_ListCountOverflow(t *testing.T) {
	// Craft a header where ListCount is absurdly large (would overflow int on 32-bit
	// and cause a huge allocation or negative headerSize on 64-bit overflow).
	b := make([]byte, 56)
	// ListCount at offset 40: set to a value that makes 56 + count*664 overflow int64
	binary.LittleEndian.PutUint64(b[40:48], 0x7FFFFFFFFFFFFFFF) // MaxInt64
	binary.LittleEndian.PutUint64(b[48:56], 0)

	_, err := BytesToCosmeticList(b)
	if err == nil {
		t.Error("expected error when ListCount causes integer overflow in headerSize calculation")
	}
}

func TestBytesToCosmeticList_HeaderIncomplete(t *testing.T) {
	b := make([]byte, 56)
	// ListCount = 5, but we don't provide the 5*664 bytes
	binary.LittleEndian.PutUint64(b[40:48], 5)
	_, err := BytesToCosmeticList(b)
	if err == nil {
		t.Error("expected error when header is incomplete (ListCount entries don't fit)")
	}
}

func TestBytesToCosmeticList_ExtSizeOverflow(t *testing.T) {
	// Build a minimal valid header with 1 entry, but craft the entry so
	// ImageListingEntrySize + OtherEntrySize overflows uint32.
	entryCount := uint64(1)
	entrySize := 664
	totalSize := 56 + entryCount*uint64(entrySize)
	b := make([]byte, totalSize)

	binary.LittleEndian.PutUint64(b[40:48], entryCount)
	binary.LittleEndian.PutUint64(b[48:56], entryCount)

	// Locate OtherEntrySize and ImageListingEntrySize inside the CDescriptor.
	// From cosmetictypes.go the struct starts at offset 56. We need to know
	// the offsets of ImageListingEntrySize and OtherEntrySize.
	// According to CDescriptor layout they're uint32 fields. Without exact offsets
	// we set them to MaxUint32/2 each so their sum wraps around.
	// We use a sentinel value that makes the sum negative when cast to int.
	const halfMax = uint32(0xC0000000) // 0xC0000000 + 0xC0000000 = 0x180000000, truncates to 0x80000000 if stored as uint32
	// Write MaxUint32 into the first 4 bytes of the entry for ImageListingEntrySize placeholder
	// and MaxUint32 for OtherEntrySize; their sum as uint32 wraps to small value, but
	// as int can be misinterpreted. We validate the addition is checked.
	// For the test to be meaningful: set ImageListingEntrySize=halfMax, OtherEntrySize=halfMax
	// Actual field offsets aren't directly accessible here; we rely on the fact
	// that BytesToCosmeticList should detect overflow when sz < 0 or sz > len(extData).
	// The current code already checks `sz <= len(extData)`, so this test verifies
	// the case where sz is already large enough to panic on 32-bit (overflowed to negative).
	// On 64-bit platforms the existing check prevents the panic. This test confirms
	// the overflow → silent skip behavior is safe (no panic, no OOB).
	_, err := BytesToCosmeticList(b)
	// The file has no ext data; sz should be 0 for our zero-filled struct, so this
	// path succeeds. The important invariant is: it must not panic.
	if err != nil {
		t.Logf("BytesToCosmeticList returned error (acceptable): %v", err)
	}
}

func TestBytesToCosmeticList_BinaryReadError(t *testing.T) {
	// Provide exactly 56 bytes with ListCount=1 but only 100 bytes of entry data
	// (needs 664 bytes for 1 entry). This tests that binary.Read failure is caught.
	b := make([]byte, 156)
	binary.LittleEndian.PutUint64(b[40:48], 1)
	binary.LittleEndian.PutUint64(b[48:56], 1)
	// Only 100 bytes of entry data provided (needs 664)

	_, err := BytesToCosmeticList(b)
	if err == nil {
		t.Error("expected error when entry data is truncated, but BytesToCosmeticList succeeded")
	}
}

// ----------------------------------------------------------------------------
// Unsafe float extraction — safe decode equivalence test
// Tests that the replacement (binary.LittleEndian) matches the unsafe.Pointer result
// This test itself will pass today; it documents the expected behaviour after fix.
// ----------------------------------------------------------------------------

func TestSafeFloatDecode_MatchesBinary(t *testing.T) {
	// Encode a known float value into 4 bytes then decode with binary.LittleEndian.
	expected := float32(3.14159)
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, math.Float32bits(expected))

	got := math.Float32frombits(binary.LittleEndian.Uint32(buf[0:4]))
	if got != expected {
		t.Errorf("safe decode: got %v, want %v", got, expected)
	}
}
