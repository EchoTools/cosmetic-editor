package data

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

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

func BytesToCosmeticList(b []byte) (CosmeticList, error) {
	var cList CosmeticList
	if len(b) < 56 {
		return cList, fmt.Errorf("file too small")
	}
	cList.ListSize = binary.LittleEndian.Uint64(b[8:16])
	cList.Unk1 = binary.LittleEndian.Uint32(b[28:32])
	cList.ListCount = binary.LittleEndian.Uint64(b[40:48])
	cList.ListCount2 = binary.LittleEndian.Uint64(b[48:56])
	headerSize := 56 + int(cList.ListCount)*664
	if len(b) < headerSize {
		return cList, fmt.Errorf("header incomplete")
	}
	extData := b[headerSize:]
	cList.CosmeticEntries = make([]CosmeticEntry, cList.ListCount)
	reader := bytes.NewReader(b[56:headerSize])
	for i := 0; i < int(cList.ListCount); i++ {
		binary.Read(reader, binary.LittleEndian, &cList.CosmeticEntries[i].CEntry)
		sz := int(cList.CosmeticEntries[i].CEntry.ImageListingEntrySize + cList.CosmeticEntries[i].CEntry.OtherEntrySize)
		if sz > 0 && sz <= len(extData) {
			cList.CosmeticEntries[i].CEntryExtData = make([]byte, sz)
			copy(cList.CosmeticEntries[i].CEntryExtData, extData[:sz])
			extData = extData[sz:]
		}
	}
	return cList, nil
}

func CosmeticListToBytes(cList CosmeticList) ([]byte, error) {
	// Dynamically update header metadata based on actual slice length
	cList.ListCount = uint64(len(cList.CosmeticEntries))
	cList.ListCount2 = cList.ListCount
	cList.ListSize = cList.ListCount * 664

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
