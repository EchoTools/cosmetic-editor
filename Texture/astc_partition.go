package texture

// Partition pattern generation for multi-partition ASTC blocks.
//
// ASTC does not store a partition map. It stores a 10-bit seed, and both the
// encoder and the decoder run the same hash to regenerate which partition each
// texel belongs to. The hash below is the one defined by the ASTC
// specification; changing any constant changes every multi-partition texture.
//
// Echo VR's shipped Quest textures are all single-partition, so nothing here
// runs on them. It exists so that a texture from another source, or a future
// build, still decodes.

// hash52 is the ASTC partition hash.
func hash52(v uint32) uint32 {
	v ^= v >> 15
	v *= 0xEEDE0891
	v ^= v >> 5
	v += v << 16
	v ^= v >> 7
	v ^= v >> 3
	v ^= v << 6
	v ^= v >> 17
	return v
}

// selectPartition returns which partition a texel belongs to.
//
// smallBlock biases the coordinates for blocks with fewer than 32 texels, which
// spreads the partitions more evenly at small footprints.
func selectPartition(seed, x, y, partitionCount int, smallBlock bool) int {
	if smallBlock {
		x <<= 1
		y <<= 1
	}
	seed += (partitionCount - 1) * 1024
	rnum := hash52(uint32(seed))

	s := [12]int{}
	shifts := [12]uint{0, 4, 8, 12, 16, 20, 24, 28, 18, 22, 26, 30}
	for i, sh := range shifts {
		var v int
		if i == 11 {
			v = int((rnum>>30 | rnum<<2) & 0xF)
		} else {
			v = int((rnum >> sh) & 0xF)
		}
		s[i] = v * v // squaring biases the distribution toward low values
	}

	var sh1, sh2 uint
	if seed&1 != 0 {
		sh1 = 5
		if seed&2 != 0 {
			sh1 = 4
		}
		sh2 = 5
		if partitionCount == 3 {
			sh2 = 6
		}
	} else {
		sh1 = 5
		if partitionCount == 3 {
			sh1 = 6
		}
		sh2 = 5
		if seed&2 != 0 {
			sh2 = 4
		}
	}
	sh3 := sh2
	if seed&0x10 != 0 {
		sh3 = sh1
	}

	seed1 := s[0] >> sh1
	seed2 := s[1] >> sh2
	seed3 := s[2] >> sh1
	seed4 := s[3] >> sh2
	seed5 := s[4] >> sh1
	seed6 := s[5] >> sh2
	seed7 := s[6] >> sh1
	seed8 := s[7] >> sh2
	seed9 := s[8] >> sh3
	seed10 := s[9] >> sh3
	seed11 := s[10] >> sh3
	seed12 := s[11] >> sh3
	_, _, _ = seed9, seed10, seed11
	_ = seed12

	a := seed1*x + seed2*y + int(rnum>>14)
	b := seed3*x + seed4*y + int(rnum>>10)
	c := seed5*x + seed6*y + int(rnum>>6)
	d := seed7*x + seed8*y + int(rnum>>2)

	a &= 0x3F
	b &= 0x3F
	c &= 0x3F
	d &= 0x3F

	if partitionCount <= 3 {
		d = 0
	}
	if partitionCount <= 2 {
		c = 0
	}
	if partitionCount <= 1 {
		b = 0
	}

	switch {
	case a >= b && a >= c && a >= d:
		return 0
	case b >= c && b >= d:
		return 1
	case c >= d:
		return 2
	default:
		return 3
	}
}
