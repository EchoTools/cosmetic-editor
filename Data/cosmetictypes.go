package data

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"strings"
)

type CDescriptor struct { // Total size is 664 bytes
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
	BannerMedalHeight   float32
	BannerMedalWidth    float32 // maybe one of these is rotation?
	BannerEmblemXPos    float32
	BannerEmblemYPos    float32
	BannerEmblemHeight  float32
	BannerEmblemWidth   float32 // maybe one of these is rotation?

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

func NewCDescriptor() CDescriptor {
	foo := CDescriptor{}
	foo.ConstSymbol = 5418741735304881729
	foo.Unused1 = 4294967295
	foo.RaritySymbol = HexToSymbol(RarityDefault)
	foo.UnknownSymbol3 = HexToSymbol(RarityDefault)
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

type CosmeticEntry struct {
	CEntry        CDescriptor
	CEntryExtData []byte // unfixed size, see CEntry.ImageListingEntrySize / CEntry.OtherEntrySize
}

const (
	RarityDefault   = "2fd4426500000000"
	RarityCommon    = "41ca12d7c5ed24b0"
	RarityLegendary = "3a5cd587c6742add"
	RarityEpic      = "c8b4172f3d178ead"
	RaritySuperb    = "41ca22cc610538bc"
	RarityFine      = "c8b4172f3a213335"
	RarityMythic    = "41ca18d7f4be43bd"
)

// abstract all non-cosmetic-specific data away in these structs
// need functions to/from cosmeticEntry{} here. user should never have to manipulate cosmeticEntry directly, only through below structs & functions

type CBooster struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string
}

// i believe currency & xp boosts are handled individually by the websocket services, and have no specific data past a cosmetic entry
type CCurrency struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string
}

type CXp_boost_individual struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string
}

type CXp_boost_party struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string
}

type CBracer struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string
}

type CChassis struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string
	TextureSymbol   string
}

func (c *CChassis) ToCosmeticEntry() (CosmeticEntry, error) {
	foo := CosmeticEntry{}
	foo.CEntry = NewCDescriptor()
	foo.CEntry.CosmeticTypeSymbol = int64(ToSymbol("chassis"))
	foo.CEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol
	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))
	foo.CEntry.RaritySymbol = HexToSymbol(c.Rarity)
	foo.CEntry.ThumbnailSymbol = HexToSymbol(c.ThumbnailSymbol)
	foo.CEntry.TextureSymbol = HexToSymbol(c.TextureSymbol)
	return foo, nil
}

func (c *CChassis) FromCosmeticEntry(d CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = SymbolToHex(d.CEntry.RaritySymbol)
	c.ThumbnailSymbol = SymbolToHex(d.CEntry.ThumbnailSymbol)
	c.TextureSymbol = SymbolToHex(d.CEntry.TextureSymbol)
	return nil
}

type CMedal struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string
	TextureSymbol   string
}

func (c *CMedal) ToCosmeticEntry() (CosmeticEntry, error) {
	foo := CosmeticEntry{}
	foo.CEntry = NewCDescriptor()
	foo.CEntry.CosmeticTypeSymbol = int64(ToSymbol("medal"))
	foo.CEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol
	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))
	foo.CEntry.RaritySymbol = HexToSymbol(c.Rarity)
	foo.CEntry.ThumbnailSymbol = HexToSymbol(c.ThumbnailSymbol)
	foo.CEntry.TextureSymbol = HexToSymbol(c.TextureSymbol)
	return foo, nil
}

func (c *CMedal) FromCosmeticEntry(d CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = SymbolToHex(d.CEntry.RaritySymbol)
	c.ThumbnailSymbol = SymbolToHex(d.CEntry.ThumbnailSymbol)
	c.TextureSymbol = SymbolToHex(d.CEntry.TextureSymbol)
	return nil
}

type CEmblem struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string
	TextureSymbol   string
}

func (c *CEmblem) ToCosmeticEntry() (CosmeticEntry, error) {
	foo := CosmeticEntry{}
	foo.CEntry = NewCDescriptor()
	foo.CEntry.CosmeticTypeSymbol = int64(ToSymbol("decal"))
	foo.CEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol
	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))
	foo.CEntry.RaritySymbol = HexToSymbol(c.Rarity)
	foo.CEntry.ThumbnailSymbol = HexToSymbol(c.ThumbnailSymbol)
	foo.CEntry.TextureSymbol = HexToSymbol(c.TextureSymbol)
	return foo, nil
}

func (c *CEmblem) FromCosmeticEntry(d CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = SymbolToHex(d.CEntry.RaritySymbol)
	c.ThumbnailSymbol = SymbolToHex(d.CEntry.ThumbnailSymbol)
	c.TextureSymbol = SymbolToHex(d.CEntry.TextureSymbol)
	return nil
}

type CBanner struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string

	TextureSymbol string

	MedalXPos   float32
	MedalYPos   float32
	MedalHeight float32
	MedalWidth  float32

	EmblemXPos   float32
	EmblemYPos   float32
	EmblemHeight float32
	EmblemWidth  float32
}

func (c *CBanner) ToCosmeticEntry() (CosmeticEntry, error) {
	foo := CosmeticEntry{}
	foo.CEntry = NewCDescriptor() // init default values

	foo.CEntry.CosmeticTypeSymbol = int64(ToSymbol("banner"))

	foo.CEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol

	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))

	foo.CEntry.RaritySymbol = HexToSymbol(c.Rarity)
	foo.CEntry.ThumbnailSymbol = HexToSymbol(c.ThumbnailSymbol)
	foo.CEntry.TextureSymbol = HexToSymbol(c.TextureSymbol)

	foo.CEntry.BannerMedalXPos = c.MedalXPos
	foo.CEntry.BannerMedalYPos = c.MedalYPos
	foo.CEntry.BannerMedalHeight = c.MedalHeight
	foo.CEntry.BannerMedalWidth = c.MedalWidth

	foo.CEntry.BannerEmblemXPos = c.EmblemXPos
	foo.CEntry.BannerEmblemYPos = c.EmblemYPos
	foo.CEntry.BannerEmblemHeight = c.EmblemHeight
	foo.CEntry.BannerEmblemWidth = c.EmblemWidth

	return foo, nil
}

func (c *CBanner) FromCosmeticEntry(d CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = SymbolToHex(d.CEntry.RaritySymbol)
	c.ThumbnailSymbol = SymbolToHex(d.CEntry.ThumbnailSymbol)
	c.TextureSymbol = SymbolToHex(d.CEntry.TextureSymbol)

	c.MedalXPos = d.CEntry.BannerMedalXPos
	c.MedalYPos = d.CEntry.BannerMedalYPos
	c.MedalHeight = d.CEntry.BannerMedalHeight
	c.MedalWidth = d.CEntry.BannerMedalWidth

	c.EmblemXPos = d.CEntry.BannerEmblemXPos
	c.EmblemYPos = d.CEntry.BannerEmblemYPos
	c.EmblemHeight = d.CEntry.BannerEmblemHeight
	c.EmblemWidth = d.CEntry.BannerEmblemWidth

	return nil
}

type CDecal struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string
	TextureSymbol   string
}

func (c *CDecal) ToCosmeticEntry() (CosmeticEntry, error) {
	foo := CosmeticEntry{}
	foo.CEntry = NewCDescriptor()
	foo.CEntry.CosmeticTypeSymbol = int64(ToSymbol("decal"))
	foo.CEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol
	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))
	foo.CEntry.RaritySymbol = HexToSymbol(c.Rarity)
	foo.CEntry.ThumbnailSymbol = HexToSymbol(c.ThumbnailSymbol)
	foo.CEntry.TextureSymbol = HexToSymbol(c.TextureSymbol)
	return foo, nil
}

func (c *CDecal) FromCosmeticEntry(d CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = SymbolToHex(d.CEntry.RaritySymbol)
	c.ThumbnailSymbol = SymbolToHex(d.CEntry.ThumbnailSymbol)
	c.TextureSymbol = SymbolToHex(d.CEntry.TextureSymbol)
	return nil
}

type CTag struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string
	TextureSymbol   string
}

func (c *CTag) ToCosmeticEntry() (CosmeticEntry, error) {
	foo := CosmeticEntry{}
	foo.CEntry = NewCDescriptor()
	foo.CEntry.CosmeticTypeSymbol = int64(ToSymbol("tag"))
	foo.CEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol
	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))
	foo.CEntry.RaritySymbol = HexToSymbol(c.Rarity)
	foo.CEntry.ThumbnailSymbol = HexToSymbol(c.ThumbnailSymbol)
	foo.CEntry.TextureSymbol = HexToSymbol(c.TextureSymbol)
	return foo, nil
}

func (c *CTag) FromCosmeticEntry(d CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = SymbolToHex(d.CEntry.RaritySymbol)
	c.ThumbnailSymbol = SymbolToHex(d.CEntry.ThumbnailSymbol)
	c.TextureSymbol = SymbolToHex(d.CEntry.TextureSymbol)
	return nil
}

type CPip struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string
	TextureSymbol   string
}

func (c *CPip) ToCosmeticEntry() (CosmeticEntry, error) {
	foo := CosmeticEntry{}
	foo.CEntry = NewCDescriptor()
	foo.CEntry.CosmeticTypeSymbol = int64(ToSymbol("pip"))
	foo.CEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol
	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))
	foo.CEntry.RaritySymbol = HexToSymbol(c.Rarity)
	foo.CEntry.ThumbnailSymbol = HexToSymbol(c.ThumbnailSymbol)
	foo.CEntry.TextureSymbol = HexToSymbol(c.TextureSymbol)
	return foo, nil
}

func (c *CPip) FromCosmeticEntry(d CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = SymbolToHex(d.CEntry.RaritySymbol)
	c.ThumbnailSymbol = SymbolToHex(d.CEntry.ThumbnailSymbol)
	c.TextureSymbol = SymbolToHex(d.CEntry.TextureSymbol)
	return nil
}

type CDecalborder struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string
}

type CGoal_fx struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string

	AssetSymbol1 string // not in assets
	AssetSymbol2 string // in assets, soundbank pointer(?) this file has the soundbank typesymbol:namesymbol at byte 8
}

// RGB colors stored as float32 values between 0.0 and 1.0
// max number of colors to cycle through hasn't been checked, 3 is highest seen in official emissives
type CEmissive struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string
	TextureSymbol   string

	Unk1   float32 // something to do with scrolling???
	Unk2   float32 // something to do with scrolling???
	Colors [][3]float32
}

func (c *CEmissive) ToCosmeticEntry() (CosmeticEntry, error) {
	foo := CosmeticEntry{}
	foo.CEntry = NewCDescriptor() // init default values

	foo.CEntry.CosmeticTypeSymbol = int64(ToSymbol("emissive"))

	foo.CEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol

	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))

	foo.CEntry.RaritySymbol = HexToSymbol(c.Rarity)

	foo.CEntry.ThumbnailSymbol = HexToSymbol(c.ThumbnailSymbol)
	foo.CEntry.TextureSymbol = HexToSymbol(c.TextureSymbol)
	foo.CEntry.EmissiveUnk1 = c.Unk1
	foo.CEntry.EmissiveUnk2 = c.Unk2

	buf := make([]byte, len(c.Colors)*12)

	for i := 0; i < len(c.Colors); i++ {
		binary.LittleEndian.PutUint32(buf[i*12:], math.Float32bits(c.Colors[i][0]))
		binary.LittleEndian.PutUint32(buf[i*12+4:], math.Float32bits(c.Colors[i][1]))
		binary.LittleEndian.PutUint32(buf[i*12+8:], math.Float32bits(c.Colors[i][2]))
	}

	foo.CEntryExtData = buf

	foo.CEntry.ExtDataParamCount = int64(len(c.Colors))
	foo.CEntry.ExtDataParamCount2 = foo.CEntry.ExtDataParamCount
	foo.CEntry.OtherEntrySize = int64(len(c.Colors) * 12)

	return foo, nil
}

func (c *CEmissive) FromCosmeticEntry(d CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = SymbolToHex(d.CEntry.RaritySymbol)
	c.ThumbnailSymbol = SymbolToHex(d.CEntry.ThumbnailSymbol)
	c.TextureSymbol = SymbolToHex(d.CEntry.TextureSymbol)
	c.Unk1 = d.CEntry.EmissiveUnk1
	c.Unk2 = d.CEntry.EmissiveUnk2

	// emissive-specific conversions
	Colors := make([][3]float32, d.CEntry.ExtDataParamCount)
	if len(d.CEntryExtData) != int(d.CEntry.ExtDataParamCount)*12 {
		c = nil
		return errors.New("invalid extdata size for emissive cosmetic")
	}
	for i := 0; i < int(d.CEntry.ExtDataParamCount); i++ {
		Colors[i][0] = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[i*12 : i*12+4]))
		Colors[i][1] = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[i*12+4 : i*12+8]))
		Colors[i][2] = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[i*12+8 : i*12+12]))
	}
	c.Colors = Colors

	return nil
}

type CEmote struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string
	Framerate       uint32 // unverified, but emotes run at 15fps, this value is only set in emotes, and is always set to 15.

	Unk1        uint32
	EmoteFrames []string // stored in ExtData
}

func (c *CEmote) ToCosmeticEntry() (CosmeticEntry, error) {
	foo := CosmeticEntry{}
	foo.CEntry = NewCDescriptor() // init default values

	foo.CEntry.CosmeticTypeSymbol = int64(ToSymbol("emote"))

	foo.CEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol

	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))

	foo.CEntry.RaritySymbol = HexToSymbol(c.Rarity)

	foo.CEntry.ThumbnailSymbol = HexToSymbol(c.ThumbnailSymbol)

	foo.CEntry.ImageListingEntrySize = int64(len(c.EmoteFrames) * 8)
	foo.CEntry.EmoteFrameCount = int64(len(c.EmoteFrames))
	foo.CEntry.EmoteFrameCount2 = foo.CEntry.EmoteFrameCount

	foo.CEntry.EmoteUnk2 = c.Framerate

	foo.CEntry.EmoteUnk1 = c.Unk1 // unknown parameter

	foo.CEntry.UnknownSymbol9 = 8287024585674271749 // ??? hardcoded, doesn't seem to change between all emotes

	foo.CEntryExtData = make([]byte, len(c.EmoteFrames)*8)

	// c.EmoteFrames -> []byte -> foo.CEntryExtData
	for i := 0; i < len(c.EmoteFrames); i++ {
		binary.LittleEndian.PutUint64(foo.CEntryExtData[i*8:], uint64(HexToSymbol(c.EmoteFrames[i])))
	}

	return foo, nil
}

func (c *CEmote) FromCosmeticEntry(d CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = SymbolToHex(d.CEntry.RaritySymbol)
	c.ThumbnailSymbol = SymbolToHex(d.CEntry.ThumbnailSymbol)

	c.Framerate = d.CEntry.EmoteUnk2

	c.Unk1 = d.CEntry.EmoteUnk1

	c.EmoteFrames = make([]string, len(d.CEntryExtData)/8)

	for i := 0; i < len(d.CEntryExtData); i += 8 {
		c.EmoteFrames[i/8] = SymbolToHex(int64(binary.LittleEndian.Uint64(d.CEntryExtData[i : i+8])))
	}
	return nil
}

type CPattern struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string

	TextureSymbol string
}

func (c *CPattern) ToCosmeticEntry() (CosmeticEntry, error) {
	foo := CosmeticEntry{}
	foo.CEntry = NewCDescriptor() // init default values
	foo.CEntry.CosmeticTypeSymbol = int64(ToSymbol("pattern"))
	foo.CEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol
	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))
	foo.CEntry.RaritySymbol = HexToSymbol(c.Rarity)
	foo.CEntry.ThumbnailSymbol = HexToSymbol(c.ThumbnailSymbol)
	foo.CEntry.TextureSymbol = HexToSymbol(c.TextureSymbol)
	return foo, nil
}

func (c *CPattern) FromCosmeticEntry(d CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = SymbolToHex(d.CEntry.RaritySymbol)
	c.ThumbnailSymbol = SymbolToHex(d.CEntry.ThumbnailSymbol)
	c.TextureSymbol = SymbolToHex(d.CEntry.TextureSymbol)
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
	Rarity          string
	ThumbnailSymbol string

	PrimaryColor_R   float32 // stored in ExtData
	PrimaryColor_G   float32 // stored in ExtData
	PrimaryColor_B   float32 // stored in ExtData
	SecondaryColor_R float32 // stored in ExtData
	SecondaryColor_G float32 // stored in ExtData
	SecondaryColor_B float32 // stored in ExtData
}

func (c *CTint) ToCosmeticEntry() (CosmeticEntry, error) {
	foo := CosmeticEntry{}
	foo.CEntry = NewCDescriptor() // init default values

	foo.CEntry.CosmeticTypeSymbol = int64(ToSymbol("tint"))

	foo.CEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol

	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))

	foo.CEntry.RaritySymbol = HexToSymbol(c.Rarity)

	foo.CEntry.ThumbnailSymbol = HexToSymbol(c.ThumbnailSymbol)

	buf := make([]byte, 24)
	binary.LittleEndian.PutUint32(buf[0:], math.Float32bits(c.PrimaryColor_R))
	binary.LittleEndian.PutUint32(buf[4:], math.Float32bits(c.PrimaryColor_G))
	binary.LittleEndian.PutUint32(buf[8:], math.Float32bits(c.PrimaryColor_B))
	binary.LittleEndian.PutUint32(buf[12:], math.Float32bits(c.SecondaryColor_R))
	binary.LittleEndian.PutUint32(buf[16:], math.Float32bits(c.SecondaryColor_G))
	binary.LittleEndian.PutUint32(buf[20:], math.Float32bits(c.SecondaryColor_B))
	foo.CEntryExtData = buf

	foo.CEntry.OtherEntrySize = int64(len(foo.CEntryExtData)) // always 72 for tints(?)
	foo.CEntry.ExtDataParamCount = 2                          // always 2 for tints
	foo.CEntry.ExtDataParamCount2 = 2                         // always 2 for tints

	return foo, nil
}

func (c *CTint) FromCosmeticEntry(d CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = SymbolToHex(d.CEntry.RaritySymbol)
	c.ThumbnailSymbol = SymbolToHex(d.CEntry.ThumbnailSymbol)

	if len(d.CEntryExtData) != 24 {
		c = nil
		return errors.New("invalid extdata size for tint cosmetic")
	}
	// tint-specific conversions
	c.PrimaryColor_R = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[0:4]))
	c.PrimaryColor_G = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[4:8]))
	c.PrimaryColor_B = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[8:12]))
	c.SecondaryColor_R = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[12:16]))
	c.SecondaryColor_G = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[16:20]))
	c.SecondaryColor_B = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[20:24]))

	return nil
}

type CTitle struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string

	TitleString string
}

func (c *CTitle) ToCosmeticEntry() (CosmeticEntry, error) {
	foo := CosmeticEntry{}
	foo.CEntry = NewCDescriptor() // init default values
	foo.CEntry.CosmeticTypeSymbol = int64(ToSymbol("title"))
	foo.CEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol
	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))
	foo.CEntry.RaritySymbol = HexToSymbol(c.Rarity)
	foo.CEntry.ThumbnailSymbol = HexToSymbol(c.ThumbnailSymbol)
	copy(foo.CEntry.TitleString[:], []byte(c.TitleString))
	return foo, nil
}

func (c *CTitle) FromCosmeticEntry(d CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = SymbolToHex(d.CEntry.RaritySymbol)
	c.ThumbnailSymbol = SymbolToHex(d.CEntry.ThumbnailSymbol)
	c.TitleString = string(bytes.TrimRight(d.CEntry.TitleString[:], "\x00"))
	return nil
}

type CFanfare struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          string
	ThumbnailSymbol string
	Fanfare1ID      uint32
	Fanfare2ID      uint32
}

func (c *CFanfare) ToCosmeticEntry() (CosmeticEntry, error) {
	foo := CosmeticEntry{}
	foo.CEntry = NewCDescriptor() // init default values

	// A bit of a hack: Fanfares don't have their own type symbol.
	// We just use the tint symbol as a base and the presence of sound IDs will identify it.
	foo.CEntry.CosmeticTypeSymbol = int64(ToSymbol("tint"))

	foo.CEntry.InternalNameSymbol = int64(ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol

	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))

	foo.CEntry.RaritySymbol = HexToSymbol(c.Rarity)
	foo.CEntry.WWiseSoundBankID1 = c.Fanfare1ID
	foo.CEntry.WWiseSoundBankID2 = c.Fanfare2ID
	foo.CEntry.ThumbnailSymbol = HexToSymbol(c.ThumbnailSymbol)

	return foo, nil
}

func (c *CFanfare) FromCosmeticEntry(d CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = SymbolToHex(d.CEntry.RaritySymbol)
	c.Fanfare1ID = d.CEntry.WWiseSoundBankID1
	c.Fanfare2ID = d.CEntry.WWiseSoundBankID2
	c.ThumbnailSymbol = SymbolToHex(d.CEntry.ThumbnailSymbol)
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
