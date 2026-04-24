package data

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// CosmeticList represents the full binary cosmetic database parsed from the game's data file.
type CosmeticList struct {
	_               [8]byte
	ListSize        uint64
	_               [12]byte
	Unk1            uint32
	_               [8]byte
	ListCount       uint64
	ListCount2      uint64
	CosmeticEntries []CosmeticEntry
}

// maxCosmeticEntries is a sanity cap that prevents maliciously large allocations.
const maxCosmeticEntries = 1 << 16 // 65 536 entries is far more than any real file needs

// BytesToCosmeticList deserializes a raw byte slice into a CosmeticList.
func BytesToCosmeticList(b []byte) (CosmeticList, error) {
	var cList CosmeticList
	if len(b) < 56 {
		return cList, fmt.Errorf("file too small")
	}
	cList.ListSize = binary.LittleEndian.Uint64(b[8:16])
	cList.Unk1 = binary.LittleEndian.Uint32(b[28:32])
	cList.ListCount = binary.LittleEndian.Uint64(b[40:48])
	cList.ListCount2 = binary.LittleEndian.Uint64(b[48:56])

	// Guard against integer overflow: 56 + count*664 must fit in a positive int.
	if cList.ListCount > maxCosmeticEntries {
		return cList, fmt.Errorf("ListCount %d exceeds maximum allowed (%d)", cList.ListCount, maxCosmeticEntries)
	}
	headerSize := 56 + int(cList.ListCount)*664
	if headerSize < 56 || len(b) < headerSize {
		return cList, fmt.Errorf("header incomplete")
	}
	extData := b[headerSize:]
	cList.CosmeticEntries = make([]CosmeticEntry, cList.ListCount)
	reader := bytes.NewReader(b[56:headerSize])
	for i := 0; i < int(cList.ListCount); i++ {
		if err := binary.Read(reader, binary.LittleEndian, &cList.CosmeticEntries[i].CEntry); err != nil {
			return cList, fmt.Errorf("reading entry %d: %w", i, err)
		}
		imgSz := cList.CosmeticEntries[i].CEntry.ImageListingEntrySize
		othSz := cList.CosmeticEntries[i].CEntry.OtherEntrySize
		// Guard against negative sizes and int64 overflow before converting to int.
		if imgSz <= 0 || othSz < 0 || imgSz > int64(^uint(0)>>1)-othSz {
			continue
		}
		sz := int(imgSz + othSz)
		if sz > 0 && sz <= len(extData) {
			cList.CosmeticEntries[i].CEntryExtData = make([]byte, sz)
			copy(cList.CosmeticEntries[i].CEntryExtData, extData[:sz])
			extData = extData[sz:]
		}
	}
	return cList, nil
}

// CosmeticListToBytes serializes a CosmeticList back to the binary format expected by the game.
func CosmeticListToBytes(cList CosmeticList) ([]byte, error) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, [8]byte{})
	binary.Write(&buf, binary.LittleEndian, cList.ListSize)
	binary.Write(&buf, binary.LittleEndian, [12]byte{})
	binary.Write(&buf, binary.LittleEndian, cList.Unk1)
	binary.Write(&buf, binary.LittleEndian, [8]byte{})
	binary.Write(&buf, binary.LittleEndian, cList.ListCount)
	binary.Write(&buf, binary.LittleEndian, cList.ListCount2)
	for i := 0; i < len(cList.CosmeticEntries); i++ {
		binary.Write(&buf, binary.LittleEndian, cList.CosmeticEntries[i].CEntry)
	}
	for i := 0; i < len(cList.CosmeticEntries); i++ {
		buf.Write(cList.CosmeticEntries[i].CEntryExtData)
	}
	return buf.Bytes(), nil
}
