package data

import "testing"

// TestTypeHashesMatchVerifiedValues pins every derived hash against the values
// confirmed from the shipped binaries (quest_combat_port/data/hash_lookup.json,
// built from libr15.so / echovr.exe disassembly).  If ToSymbol ever drifts,
// this catches it before the app starts writing files the game cannot load.
func TestTypeHashesMatchVerifiedValues(t *testing.T) {
	cases := []struct {
		base     string
		pc, ques string
	}{
		{"CGTextureResource", "4a4c32c49300b8a0", "e2efe7289d5985b8"},
		{"CR15NetRewardItemCR", "32f30fe361939dee", "24cbfd54e9a7f2ea"},
		{"CGMeshListResource", "4e426f88c1b5d7ac", "616671cd1c9b4046"},
		{"CGInstancedModelResource", "37102e4b27955a14", "038c44cbea5c59e8"},
		{"CGMaterialResource", "e3e0266f1911dafa", "685a78f51339749c"},
		{"CGTextureStreamingResource", "c2434c5a99e139ce", "1396ddab0094e9e2"},
		{"CArchiveResource", "2a41cf1c1d9e5d32", "004c5273be3eab58"},
	}
	for _, c := range cases {
		if got := PlatformPC.TypeHash(c.base); got != c.pc {
			t.Errorf("%sWin10 = %s, want %s", c.base, got, c.pc)
		}
		if got := PlatformQuest.TypeHash(c.base); got != c.ques {
			t.Errorf("%sAndroid = %s, want %s", c.base, got, c.ques)
		}
	}
}

func TestGPUTypeHashesMatchVerifiedValues(t *testing.T) {
	cases := []struct {
		base     string
		pc, ques string
	}{
		{"CGTextureResource", "beac1969cb7b8861", "489bb35d53ca50e9"},
		{"CR15NetRewardItemCR", "1bba0d8c8850de9d", "65762b4ed04bc6df"},
		{"CGMeshListResource", "e642bfb1abcf76df", "e3478176e0b181df"},
		{"CGInstancedModelResource", "e7a8ab5ceaef49cb", "ff0f08277e8c1735"},
	}
	for _, c := range cases {
		if got := PlatformPC.GPUTypeHash(c.base); got != c.pc {
			t.Errorf("%sWin10GPU = %s, want %s", c.base, got, c.pc)
		}
		if got := PlatformQuest.GPUTypeHash(c.base); got != c.ques {
			t.Errorf("%sAndroidGPU = %s, want %s", c.base, got, c.ques)
		}
	}
}

// TestCosmeticDBAssetNames pins the asset names the database is published
// under.  A Quest build ships two, and an edit written to only one of them
// applies on some headsets and silently does nothing on others, so the count
// matters as much as the hashes.
func TestCosmeticDBAssetNames(t *testing.T) {
	pc := PlatformPC.CosmeticDBAssetHashes()
	if len(pc) != 1 || pc[0] != "43934c379cf1e366" {
		t.Errorf("PC cosmetic DB assets = %v, want [43934c379cf1e366] (r14_glb_global_root)", pc)
	}

	quest := PlatformQuest.CosmeticDBAssetHashes()
	want := []string{"43934c379cf1e366", "bb75979f708e523b"}
	if len(quest) != len(want) {
		t.Fatalf("Quest cosmetic DB assets = %v, want %v", quest, want)
	}
	for i := range want {
		if quest[i] != want[i] {
			t.Errorf("Quest cosmetic DB asset %d = %s, want %s", i, quest[i], want[i])
		}
	}

	// The lowspec variant is the one the PC build does not have, and it is the
	// half that is easy to forget.
	if PlatformQuest.CosmeticDBAssetHashes()[1] == PlatformPC.CosmeticDBAssetHashes()[0] {
		t.Error("the Quest-only asset name must differ from the PC one")
	}
}
