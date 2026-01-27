package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"strings"
)

type cDescriptor struct { // Total size is 664 bytes
	InternalNameSymbol  int64    // symbol for internal name, should match internalNameString
	ConstSymbol         int64    // unsure what this is, seems to always be (0x41, 0x66, 0x6E, 0x06, 0xE6, 0x3C, 0x33, 0x4B)
	Unused1             uint32   // 0xFF 0xFF 0xFF 0xFF - in every cosmetic
	_                   [4]byte  // 0x00 0x00 0x00 0x00 - in every cosmetic
	Unk1                int64    // unsure, changes in cosmetic_438
	InternalNameSymbol2 int64    // don't know if ever differs from internalNameSymbol
	InternalNameString  [64]byte // length 64, string with padding of 0x00. Internal name of cosmetic, should match InternalNameSymbol.
	CosmeticTypeSymbol  int64    // Type symbol of cosmetic. title, emote, banner, etc
	RaritySymbol        int64    // Cosmetic rarity symbol
	UnknownSymbol3      int64    // unsure, is only ever the symbol for "default", or FF repeating.
	ThumbnailSymbol     int64    // Symbol of thumbnail asset in customization menu
	UnknownSymbol5      int64    // unsure, only differs in rwd_chassis_s11_retro_a & rwd_booster_s11_s1_a_retro
	UnknownSymbol6      int64    // unsure, mirrors AssetSymbol12
	UnknownSymbol7      int64    // unsure, mirrors AssetSymbol11
	UnknownSymbol8      int64    // unsure, only differs in rwd_bracer_arcade_s1_a
	DisplayNameString   [64]byte // length 64, string with padding of 0x00. Pretty display name shown in customization menu.
	DescriptionString   [64]byte // length 64, string with padding of 0x00. Description of cosmetic, haven't seen this used anywhere?
	WWiseSoundBankID1   uint32   // Event trigger ID for wem #1, the explosion sound
	WWiseSoundBankID2   uint32   // Event trigger ID for wem #2, the fanfare
	EmoteUnk1           uint32   // something to do with Emotes, emote will be non-functional if this is set to 0
	EmoteUnk2           uint32   // unsure, always set to 15. maybe emote framerate?
	EmissiveUnk1        float32  //
	EmissiveUnk2        float32  //
	TextureSymbol       int64    // Symbol of texture. Banner texture, Badge texture, etc. *not applicable to Emotes.* for Emissives, this is the scrolling texture.
	TitleString         [64]byte // length 64, string with padding of 0x00. for Titles, this is the actual title text.
	UnknownSymbol9      int64    // unsure, doesn't show up in assets. patterns - 2317691642036384126, emotes & decals - 8287024585674271749
	BannerMedalXPos     float32
	BannerMedalYPos     float32
	BannerMedalUnk1     float32 // maybe one of these is size?
	BannerMedalUnk2     float32 // maybe one of these is rotation?
	BannerEmblemXPos    float32
	BannerEmblemYPos    float32
	BannerEmblemUnk1    float32 // maybe one of these is size?
	BannerEmblemUnk2    float32 // maybe one of these is rotation?

	// below seems to be describing cosmetic ExtData
	Padding4              int64  // padding ??
	ImageListingEntrySize int64  // unsure
	Padding5              int64  // padding ??
	Unk5                  uint32 // unsure
	Unk6                  uint32 // unsure, seems to be always set to 1
	Unk7                  int64  // unsure, seems to be always set to 32
	EmoteFrameCount       int64
	EmoteFrameCount2      int64
	Padding6              int64
	OtherEntrySize        int64
	Unk10                 int64
	Unk11                 uint32
	Unk12                 uint32
	Unk13                 uint32
	Unk14                 uint32
	ExtDataParamCount     int64 // extdata parameter count or something?
	ExtDataParamCount2    int64 // extdata parameter count or something?

	// asset data for cosmetic (if applicable, for cosmetics with 3d models and fanfares])
	// don't know exactly what these are, but a majority of them exist in the game's assets
	AssetSymbol1  int64
	AssetSymbol2  int64
	AssetSymbol3  int64
	AssetSymbol4  int64
	AssetSymbol5  int64 // For chassis, bracers
	AssetSymbol6  int64 // For chassis, bracers
	AssetSymbol7  int64
	AssetSymbol8  int64 // For bracers
	AssetSymbol9  int64 // For bracers
	AssetSymbol10 int64
	AssetSymbol11 int64 // For chassis, bracers, boosters, goal_fx
	AssetSymbol12 int64 // For chassis, bracers, boosters, goal_fx
	AssetSymbol13 int64
	AssetSymbol14 int64 // For bracers
	AssetSymbol15 int64 // For bracers
}

func NewCDescriptor() cDescriptor {
	foo := cDescriptor{}
	foo.ConstSymbol = 5418741735304881729
	foo.Unused1 = 4294967295
	foo.RaritySymbol = RarityDefault
	foo.UnknownSymbol3 = RarityDefault
	foo.UnknownSymbol5 = -1
	foo.UnknownSymbol6 = -1
	foo.UnknownSymbol7 = -1
	foo.UnknownSymbol8 = -1
	foo.EmoteUnk1 = 65535
	foo.EmoteUnk2 = 15
	foo.TextureSymbol = -1
	foo.UnknownSymbol9 = -1
	foo.Unk6 = 1
	foo.Unk7 = 32
	foo.Unk12 = 1
	foo.Unk13 = 32
	foo.AssetSymbol1 = -1
	foo.AssetSymbol2 = -1
	foo.AssetSymbol3 = -1
	foo.AssetSymbol4 = -1
	foo.AssetSymbol5 = -1
	foo.AssetSymbol6 = -1
	foo.AssetSymbol7 = -1
	foo.AssetSymbol8 = -1
	foo.AssetSymbol9 = -1
	foo.AssetSymbol10 = -1
	foo.AssetSymbol11 = -1
	foo.AssetSymbol12 = -1
	foo.AssetSymbol13 = -1
	foo.AssetSymbol14 = -1
	foo.AssetSymbol15 = -1
	return foo
}

type cosmeticEntry struct {
	cEntry        cDescriptor
	cEntryExtData []byte // unfixed size, see cEntry.ImageListingEntrySize / cEntry.OtherEntrySize
}

const (
	RarityDefault   = int64(3446250531203406750)
	RarityCommon    = int64(4743069076990855184)
	RarityLegendary = int64(4206013803493429789)
	RarityEpic      = int64(-3980269165643360333)
	RaritySuperb    = int64(4743086626008601884)
	RarityFine      = int64(-3980269165692186443)
	RarityMythic    = int64(4743075613779889693)
)

// abstract all non-cosmetic-specific data away in these structs
// need functions to/from cosmeticEntry{} here. user should never have to manipulate cosmeticEntry directly, only through below structs & functions

type CBooster struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
}

// i believe currency & xp boosts are handled individually by the websocket services, and have no specific data past a cosmetic entry
type CCurrency struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
}

type CXp_boost_individual struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
}

type CXp_boost_party struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
}

type CBracer struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
}

type CChassis struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
}

type CMedal struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
}

type CBanner struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
}

type CDecal struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
}

type CTag struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
}

type CPip struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
}

type CDecalborder struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
}

type CGoal_fx struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64

	AssetSymbol1 int64 // not in assets
	AssetSymbol2 int64 // in assets, soundbank pointer(?) this file has the soundbank typesymbol:namesymbol at byte 8
}

// RGB colors stored as float32 values between 0.0 and 1.0
// max number of colors to cycle through hasn't been checked, 3 is highest seen in official emissives
type CEmissive struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64

	Unk1   float32 // something to do with scrolling???
	Unk2   float32 // something to do with scrolling???
	Colors [][3]float32
}

func (c *CEmissive) ToCosmeticEntry() (cosmeticEntry, error) {
	foo := cosmeticEntry{}
	foo.cEntry = NewCDescriptor() // init default values

	foo.cEntry.CosmeticTypeSymbol = int64(ToSymbol("tint"))

	foo.cEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.cEntry.InternalNameSymbol2 = foo.cEntry.InternalNameSymbol

	copy(foo.cEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.cEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.cEntry.DescriptionString[:], []byte(c.Description))

	foo.cEntry.RaritySymbol = c.Rarity

	foo.cEntry.ThumbnailSymbol = c.ThumbnailSymbol

	buf := make([]byte, 8+(len(c.Colors)*12))

	for i := 0; i < len(c.Colors); i++ {
		binary.LittleEndian.PutUint32(buf[i*12:], math.Float32bits(c.Colors[i][0]))
		binary.LittleEndian.PutUint32(buf[i*12+4:], math.Float32bits(c.Colors[i][1]))
		binary.LittleEndian.PutUint32(buf[i*12+8:], math.Float32bits(c.Colors[i][2]))
	}

	foo.cEntryExtData = buf

	foo.cEntry.ExtDataParamCount = int64(len(c.Colors))
	foo.cEntry.ExtDataParamCount2 = foo.cEntry.ExtDataParamCount
	foo.cEntry.OtherEntrySize = int64(len(c.Colors) * 12)

	return foo, nil
}

func (c *CEmissive) FromCosmeticEntry(d cosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.cEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.cEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.cEntry.DescriptionString[:], "\x00"))
	c.Rarity = d.cEntry.RaritySymbol
	c.ThumbnailSymbol = d.cEntry.ThumbnailSymbol

	// emissive-specific conversions
	Colors := make([][3]float32, d.cEntry.ExtDataParamCount)
	if len(d.cEntryExtData) != int(d.cEntry.ExtDataParamCount)*12 {
		c = nil
		return errors.New("invalid extdata size for emissive cosmetic")
	}
	for i := 0; i < int(d.cEntry.ExtDataParamCount); i++ {
		Colors[i][0] = math.Float32frombits(binary.LittleEndian.Uint32(d.cEntryExtData[i*12 : i*12+4]))
		Colors[i][1] = math.Float32frombits(binary.LittleEndian.Uint32(d.cEntryExtData[i*12+4 : i*12+8]))
		Colors[i][2] = math.Float32frombits(binary.LittleEndian.Uint32(d.cEntryExtData[i*12+8 : i*12+12]))
	}

	return nil
}

type CEmote struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
	Framerate       uint32 // unverified, but emotes run at 15fps, this value is only set in emotes, and is always set to 15.

	Unk1        uint32
	EmoteFrames []int64 // stored in ExtData
}

func (c *CEmote) ToCosmeticEntry() (cosmeticEntry, error) {
	foo := cosmeticEntry{}
	foo.cEntry = NewCDescriptor() // init default values

	foo.cEntry.CosmeticTypeSymbol = int64(ToSymbol("emote"))

	foo.cEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.cEntry.InternalNameSymbol2 = foo.cEntry.InternalNameSymbol

	copy(foo.cEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.cEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.cEntry.DescriptionString[:], []byte(c.Description))

	foo.cEntry.RaritySymbol = c.Rarity

	foo.cEntry.ThumbnailSymbol = c.ThumbnailSymbol

	foo.cEntry.ImageListingEntrySize = int64(len(c.EmoteFrames) * 8)
	foo.cEntry.EmoteFrameCount = int64(len(c.EmoteFrames))
	foo.cEntry.EmoteFrameCount2 = foo.cEntry.EmoteFrameCount

	foo.cEntry.EmoteUnk2 = c.Framerate

	foo.cEntry.EmoteUnk1 = c.Unk1 // unknown parameter

	foo.cEntry.UnknownSymbol9 = 8287024585674271749 // ??? hardcoded, doesn't seem to change between all emotes

	foo.cEntryExtData = make([]byte, len(c.EmoteFrames)*8)

	// c.EmoteFrames -> []byte -> foo.cEntryExtData
	for i := 0; i < len(c.EmoteFrames); i++ {
		binary.LittleEndian.PutUint64(foo.cEntryExtData[i*8:], uint64(c.EmoteFrames[i]))
	}

	return foo, nil
}

func (c *CEmote) FromCosmeticEntry(d cosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.cEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.cEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.cEntry.DescriptionString[:], "\x00"))
	c.Rarity = d.cEntry.RaritySymbol
	c.ThumbnailSymbol = d.cEntry.ThumbnailSymbol

	c.Framerate = d.cEntry.EmoteUnk2

	c.Unk1 = d.cEntry.EmoteUnk1

	c.EmoteFrames = make([]int64, len(d.cEntryExtData)/8)

	for i := 0; i < len(d.cEntryExtData); i += 8 {
		c.EmoteFrames[i/8] = int64(binary.BigEndian.Uint64(d.cEntryExtData[i : i+8]))
	}
	return nil
}

type CPattern struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64

	TextureSymbol int64
}

func (c *CPattern) ToCosmeticEntry() (cosmeticEntry, error) {
	foo := cosmeticEntry{}
	foo.cEntry = NewCDescriptor() // init default values
	foo.cEntry.CosmeticTypeSymbol = int64(ToSymbol("pattern"))
	foo.cEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.cEntry.InternalNameSymbol2 = foo.cEntry.InternalNameSymbol
	copy(foo.cEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.cEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.cEntry.DescriptionString[:], []byte(c.Description))
	foo.cEntry.RaritySymbol = c.Rarity
	foo.cEntry.ThumbnailSymbol = c.ThumbnailSymbol
	foo.cEntry.TextureSymbol = c.TextureSymbol
	return foo, nil
}

func (c *CPattern) FromCosmeticEntry(d cosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.cEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.cEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.cEntry.DescriptionString[:], "\x00"))
	c.Rarity = d.cEntry.RaritySymbol
	c.ThumbnailSymbol = d.cEntry.ThumbnailSymbol
	c.TextureSymbol = d.cEntry.TextureSymbol
	return nil
}

// each R, G, B value is stored as a float for some reason. i've only seen 0.15 to 0.96 range used in official tints. 0.0 to 1.0 works fine though.
// i'm assuming they know something i don't, so i'd recommend sticking to official ranges
// idk why but it's easy enough to convert from usual 0-255 rgb values, just  * / % 255
// emissives pull their "Match Tint" color from PrimaryColor_R/G/B values it seems (verify)
type CTint struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64

	PrimaryColor_R   float32 // stored in ExtData
	PrimaryColor_G   float32 // stored in ExtData
	PrimaryColor_B   float32 // stored in ExtData
	SecondaryColor_R float32 // stored in ExtData
	SecondaryColor_G float32 // stored in ExtData
	SecondaryColor_B float32 // stored in ExtData
}

func (c *CTint) ToCosmeticEntry() (cosmeticEntry, error) {
	foo := cosmeticEntry{}
	foo.cEntry = NewCDescriptor() // init default values

	foo.cEntry.CosmeticTypeSymbol = int64(ToSymbol("tint"))

	foo.cEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.cEntry.InternalNameSymbol2 = foo.cEntry.InternalNameSymbol

	copy(foo.cEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.cEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.cEntry.DescriptionString[:], []byte(c.Description))

	foo.cEntry.RaritySymbol = c.Rarity

	foo.cEntry.ThumbnailSymbol = c.ThumbnailSymbol

	buf := make([]byte, 24)
	binary.LittleEndian.PutUint32(buf[0:], math.Float32bits(c.PrimaryColor_R))
	binary.LittleEndian.PutUint32(buf[4:], math.Float32bits(c.PrimaryColor_G))
	binary.LittleEndian.PutUint32(buf[8:], math.Float32bits(c.PrimaryColor_B))
	binary.LittleEndian.PutUint32(buf[12:], math.Float32bits(c.SecondaryColor_R))
	binary.LittleEndian.PutUint32(buf[16:], math.Float32bits(c.SecondaryColor_G))
	binary.LittleEndian.PutUint32(buf[20:], math.Float32bits(c.SecondaryColor_B))
	foo.cEntryExtData = buf

	foo.cEntry.OtherEntrySize = int64(len(foo.cEntryExtData)) // always 72 for tints(?)
	foo.cEntry.ExtDataParamCount = 2                          // always 2 for tints
	foo.cEntry.ExtDataParamCount2 = 2                         // always 2 for tints

	return foo, nil
}

func (c *CTint) FromCosmeticEntry(d cosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.cEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.cEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.cEntry.DescriptionString[:], "\x00"))
	c.Rarity = d.cEntry.RaritySymbol
	c.ThumbnailSymbol = d.cEntry.ThumbnailSymbol

	if len(d.cEntryExtData) != 24 {
		c = nil
		return errors.New("invalid extdata size for tint cosmetic")
	}
	// tint-specific conversions
	c.PrimaryColor_R = math.Float32frombits(binary.LittleEndian.Uint32(d.cEntryExtData[0:4]))
	c.PrimaryColor_G = math.Float32frombits(binary.LittleEndian.Uint32(d.cEntryExtData[4:8]))
	c.PrimaryColor_B = math.Float32frombits(binary.LittleEndian.Uint32(d.cEntryExtData[8:12]))
	c.SecondaryColor_R = math.Float32frombits(binary.LittleEndian.Uint32(d.cEntryExtData[12:16]))
	c.SecondaryColor_G = math.Float32frombits(binary.LittleEndian.Uint32(d.cEntryExtData[16:20]))
	c.SecondaryColor_B = math.Float32frombits(binary.LittleEndian.Uint32(d.cEntryExtData[20:24]))

	return nil
}

type CTitle struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64

	TitleString string
}

func (c *CTitle) ToCosmeticEntry() (cosmeticEntry, error) {
	foo := cosmeticEntry{}
	foo.cEntry = NewCDescriptor() // init default values
	foo.cEntry.CosmeticTypeSymbol = int64(ToSymbol("title"))
	foo.cEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.cEntry.InternalNameSymbol2 = foo.cEntry.InternalNameSymbol
	copy(foo.cEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.cEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.cEntry.DescriptionString[:], []byte(c.Description))
	foo.cEntry.RaritySymbol = c.Rarity
	foo.cEntry.ThumbnailSymbol = c.ThumbnailSymbol
	copy(foo.cEntry.TitleString[:], []byte(c.TitleString))
	return foo, nil
}

func (c *CTitle) FromCosmeticEntry(d cosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.cEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.cEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.cEntry.DescriptionString[:], "\x00"))
	c.Rarity = d.cEntry.RaritySymbol
	c.ThumbnailSymbol = d.cEntry.ThumbnailSymbol
	c.TitleString = string(bytes.TrimRight(d.cEntry.TitleString[:], "\x00"))
	return nil
}

//func FromCosmeticEntryCommon[d cosmeticEntry, c *CTitle | *CBooster | *CCurrency | *CXp_boost_individual | *CXp_boost_party | *CBracer | *CChassis | *CMedal | *CBanner | *CDecal | *CTag | *CPip | *CDecalborder | *CGoal_fx] (error) {
//
//	var c.InternalName = string(bytes.TrimRight(d.cEntry.InternalNameString[:], "\x00"))
//	c.DisplayName = string(bytes.TrimRight(d.cEntry.DisplayNameString[:], "\x00"))
//	c.Description = string(bytes.TrimRight(d.cEntry.DescriptionString[:], "\x00"))
//	c.Rarity = d.cEntry.RaritySymbol
//	c.ThumbnailSymbol = d.cEntry.ThumbnailSymbol
//	return nil
//}
