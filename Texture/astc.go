package texture

import (
	"fmt"
	"image"
)

// A pure-Go ASTC LDR decoder, enough to preview every texture Echo VR ships on
// Quest. A census of all 1,605,814 blocks in a Quest extract found only
// single-partition blocks, no void-extent blocks, and four endpoint modes
// (6, 8, 10 and 12) across two footprints (4x4 and 8x8). Void extent and the
// remaining LDR endpoint modes are implemented anyway so that a texture outside
// that census still decodes, but the HDR modes are not: they do not occur, and
// guessing at them would be worse than reporting them.

// astcBlockSize is the byte size of one ASTC block, in every footprint.
const astcBlockSize = 16

// weightLevels maps a weight quantisation mode (0..11) to its level count.
var weightLevels = [12]int{2, 3, 4, 5, 6, 8, 10, 12, 16, 20, 24, 32}

// btq returns the bits, trits and quints that make up one value at a
// quantisation level. Every ASTC level is a power of two multiplied by 1, 3 or
// 5, so the split is derived rather than tabulated.
func btq(levels int) (bits, trits, quints int) {
	for _, m := range []struct{ mul, t, q int }{{1, 0, 0}, {3, 1, 0}, {5, 0, 1}} {
		for b := 0; b <= 8; b++ {
			if m.mul<<uint(b) == levels {
				return b, m.t, m.q
			}
		}
	}
	return 0, 0, 0
}

// iseBitCount returns how many bits a sequence of n values occupies at a
// quantisation level. Trit and quint blocks pack five and three values
// respectively into a shared field, so the cost is not simply n*bits.
func iseBitCount(n, levels int) int {
	bits, trits, quints := btq(levels)
	total := n * bits
	if trits == 1 {
		total += (n*8 + 4) / 5
	}
	if quints == 1 {
		total += (n*7 + 2) / 3
	}
	return total
}

// bitReader reads little-endian bit fields out of a block.
type bitReader struct {
	data []byte
	pos  int
}

func (r *bitReader) read(n int) uint32 {
	var v uint32
	for i := 0; i < n; i++ {
		idx := r.pos + i
		if idx >= 0 && idx>>3 < len(r.data) && r.data[idx>>3]&(1<<(uint(idx)&7)) != 0 {
			v |= 1 << uint(i)
		}
	}
	r.pos += n
	return v
}

// decodeISE decodes n values at the given quantisation level. This is the
// integer sequence encoding from the ASTC specification: each value carries a
// number of plain bits, and where the level is a multiple of three or five the
// remaining information is spread across shared trit- or quint-blocks.
func decodeISE(data []byte, bitPos, n, levels int) []uint8 {
	bits, trits, quints := btq(levels)
	out := make([]uint8, n+4) // trit/quint blocks write up to four values past n
	// A trit block covers five values and a quint block three, so the number of
	// shared blocks is n/3 at worst; size for that rather than for trits alone.
	tq := make([]uint8, n+4)

	r := &bitReader{data: data, pos: bitPos}
	lc, hc := 0, 0
	for i := 0; i < n; i++ {
		out[i] = uint8(r.read(bits))
		switch {
		case trits == 1:
			toRead := [5]int{2, 2, 1, 2, 1}
			shift := [5]int{0, 2, 4, 5, 7}
			next := [5]int{1, 2, 3, 4, 0}
			incr := [5]int{0, 0, 0, 0, 1}
			if hc < len(tq) {
				tq[hc] |= uint8(r.read(toRead[lc]) << uint(shift[lc]))
			}
			hc += incr[lc]
			lc = next[lc]
		case quints == 1:
			toRead := [3]int{3, 2, 2}
			shift := [3]int{0, 3, 5}
			next := [3]int{1, 2, 0}
			incr := [3]int{0, 0, 1}
			if hc < len(tq) {
				tq[hc] |= uint8(r.read(toRead[lc]) << uint(shift[lc]))
			}
			hc += incr[lc]
			lc = next[lc]
		}
	}

	if trits == 1 {
		for i := 0; i < (n+4)/5; i++ {
			t := tritsOfInteger[tq[i]]
			for j := 0; j < 5 && 5*i+j < len(out); j++ {
				out[5*i+j] |= t[j] << uint(bits)
			}
		}
	}
	if quints == 1 {
		for i := 0; i < (n+2)/3; i++ {
			q := quintsOfInteger[tq[i]&0x7F]
			for j := 0; j < 3 && 3*i+j < len(out); j++ {
				out[3*i+j] |= q[j] << uint(bits)
			}
		}
	}
	return out[:n]
}

// blockMode is the decoded form of an ASTC block's 11-bit mode field.
type blockMode struct {
	weightsX, weightsY int
	dualPlane          bool
	quantMode          int
	weightBits         int
}

// decodeBlockMode expands the 11-bit mode field. The weight range index is
// split across bit 4 and the low bits, which is the part most easily got wrong.
func decodeBlockMode(m uint32) (blockMode, bool) {
	var bm blockMode
	baseQuant := int((m >> 4) & 1)
	h := int((m >> 9) & 1)
	d := int((m >> 10) & 1)
	a := int((m >> 5) & 3)

	if m&3 != 0 {
		baseQuant |= int(m&3) << 1
		b := int((m >> 7) & 3)
		switch (m >> 2) & 3 {
		case 0:
			bm.weightsX, bm.weightsY = b+4, a+2
		case 1:
			bm.weightsX, bm.weightsY = b+8, a+2
		case 2:
			bm.weightsX, bm.weightsY = a+2, b+8
		case 3:
			b &= 1
			if m&0x100 != 0 {
				bm.weightsX, bm.weightsY = b+2, a+2
			} else {
				bm.weightsX, bm.weightsY = a+2, b+6
			}
		}
	} else {
		if (m>>2)&3 == 0 {
			return bm, false // reserved
		}
		baseQuant |= int((m>>2)&3) << 1
		b := int((m >> 9) & 3)
		switch (m >> 7) & 3 {
		case 0:
			bm.weightsX, bm.weightsY = 12, a+2
		case 1:
			bm.weightsX, bm.weightsY = a+2, 12
		case 2:
			bm.weightsX, bm.weightsY = a+6, b+6
			d, h = 0, 0
		case 3:
			switch (m >> 5) & 3 {
			case 0:
				bm.weightsX, bm.weightsY = 6, 10
			case 1:
				bm.weightsX, bm.weightsY = 10, 6
			default:
				return bm, false // reserved
			}
		}
	}

	bm.quantMode = (baseQuant - 2) + 6*h
	if bm.quantMode < 0 || bm.quantMode > 11 {
		return bm, false
	}
	bm.dualPlane = d != 0
	planes := 1
	if bm.dualPlane {
		planes = 2
	}
	bm.weightBits = iseBitCount(bm.weightsX*bm.weightsY*planes, weightLevels[bm.quantMode])
	return bm, true
}

// reverseBits128 mirrors a 16-byte block. ASTC stores the weight data at the
// top of the block in reverse bit order, so reversing the whole block lets the
// weights be read from offset zero with the ordinary reader.
func reverseBits128(b []byte) []byte {
	out := make([]byte, astcBlockSize)
	for i := 0; i < astcBlockSize; i++ {
		v := b[astcBlockSize-1-i]
		v = (v&0xF0)>>4 | (v&0x0F)<<4
		v = (v&0xCC)>>2 | (v&0x33)<<2
		v = (v&0xAA)>>1 | (v&0x55)<<1
		out[i] = v
	}
	return out
}

// endpointCount returns how many quantised integers an endpoint mode consumes.
func endpointCount(cem int) int { return 2 * ((cem >> 2) + 1) }

type rgba struct{ r, g, b, a int }

// blueContract is the ASTC correction applied when an endpoint pair is stored
// in swapped order, trading blue precision for red and green.
func blueContract(r, g, b, a int) rgba {
	return rgba{(r + b) >> 1, (g + b) >> 1, b, a}
}

func clamp255(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// bitTransferSigned moves the top bit of a signed offset into the base value it
// applies to, which is how the base+offset endpoint modes buy an extra bit of
// base precision. It returns the adjusted (base, offset), the offset now signed.
func bitTransferSigned(base, offset int) (int, int) {
	base |= (offset & 0x80) << 1
	offset &= 0x7F
	if offset&0x40 != 0 {
		offset -= 0x80
	}
	return base >> 1, offset >> 1
}

// deltaPair expands one base+offset endpoint pair, applying the bit transfer,
// the blue uncontraction and the endpoint swap in the order the specification
// requires. base and delta hold R, G, B and A.
func deltaPair(base, delta [4]int) (rgba, rgba) {
	var b, d [4]int
	for i := 0; i < 4; i++ {
		b[i], d[i] = bitTransferSigned(base[i], delta[i])
	}
	rgbSum := d[0] + d[1] + d[2]
	var e1 [4]int
	for i := 0; i < 4; i++ {
		e1[i] = b[i] + d[i]
	}
	c0 := rgba{b[0], b[1], b[2], b[3]}
	c1 := rgba{e1[0], e1[1], e1[2], e1[3]}
	if rgbSum < 0 {
		c0, c1 = blueContract(c1.r, c1.g, c1.b, c1.a), blueContract(c0.r, c0.g, c0.b, c0.a)
	}
	clampC := func(c rgba) rgba {
		return rgba{clamp255(c.r), clamp255(c.g), clamp255(c.b), clamp255(c.a)}
	}
	return clampC(c0), clampC(c1)
}

// decodeEndpoints expands quantised endpoint integers into a pair of LDR
// colours. Echo's Quest textures use modes 6, 8, 10 and 12 exclusively; the
// other LDR modes are here so an unusual texture still decodes.
func decodeEndpoints(cem int, v []int) (e0, e1 rgba, ok bool) {
	switch cem {
	case 0: // LDR luminance, direct
		return rgba{v[0], v[0], v[0], 255}, rgba{v[1], v[1], v[1], 255}, true
	case 1: // LDR luminance, base + offset
		l0 := (v[0] >> 2) | (v[1] & 0xC0)
		l1 := clamp255(l0 + (v[1] & 0x3F))
		return rgba{l0, l0, l0, 255}, rgba{l1, l1, l1, 255}, true
	case 4: // LDR luminance + alpha, direct
		return rgba{v[0], v[0], v[0], v[2]}, rgba{v[1], v[1], v[1], v[3]}, true
	case 5: // LDR luminance + alpha, base + offset
		l0, l1 := bitTransferSigned(v[0], v[1])
		a0, a1 := bitTransferSigned(v[2], v[3])
		l1 = clamp255(l1 + l0)
		a1 = clamp255(a1 + a0)
		return rgba{l0, l0, l0, a0}, rgba{l1, l1, l1, a1}, true
	case 6: // LDR RGB, base + scale
		return rgba{v[0] * v[3] >> 8, v[1] * v[3] >> 8, v[2] * v[3] >> 8, 255},
			rgba{v[0], v[1], v[2], 255}, true
	case 8: // LDR RGB, direct
		if v[1]+v[3]+v[5] >= v[0]+v[2]+v[4] {
			return rgba{v[0], v[2], v[4], 255}, rgba{v[1], v[3], v[5], 255}, true
		}
		return blueContract(v[1], v[3], v[5], 255), blueContract(v[0], v[2], v[4], 255), true
	case 9: // LDR RGB, base + offset
		e0, e1 := deltaPair([4]int{v[0], v[2], v[4], 255}, [4]int{v[1], v[3], v[5], 0})
		e0.a, e1.a = 255, 255
		return e0, e1, true
	case 10: // LDR RGB base + scale, with two alphas
		return rgba{v[0] * v[3] >> 8, v[1] * v[3] >> 8, v[2] * v[3] >> 8, v[4]},
			rgba{v[0], v[1], v[2], v[5]}, true
	case 12: // LDR RGBA, direct
		if v[1]+v[3]+v[5] >= v[0]+v[2]+v[4] {
			return rgba{v[0], v[2], v[4], v[6]}, rgba{v[1], v[3], v[5], v[7]}, true
		}
		return blueContract(v[1], v[3], v[5], v[7]), blueContract(v[0], v[2], v[4], v[6]), true
	case 13: // LDR RGBA, base + offset
		a, b := deltaPair([4]int{v[0], v[2], v[4], v[6]}, [4]int{v[1], v[3], v[5], v[7]})
		return a, b, true
	}
	return e0, e1, false // HDR modes: not shipped, not guessed at
}

// DecodeASTC decodes an ASTC LDR surface into an RGBA image. blockW and blockH
// are the footprint in texels.
func DecodeASTC(data []byte, width, height, blockW, blockH int) (*image.NRGBA, error) {
	if blockW <= 0 || blockH <= 0 {
		return nil, fmt.Errorf("invalid ASTC footprint %dx%d", blockW, blockH)
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid ASTC extent %dx%d", width, height)
	}
	blocksX := (width + blockW - 1) / blockW
	blocksY := (height + blockH - 1) / blockH
	if need := blocksX * blocksY * astcBlockSize; len(data) < need {
		return nil, fmt.Errorf("ASTC surface %dx%d at %dx%d needs %d bytes, got %d",
			width, height, blockW, blockH, need, len(data))
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	texels := make([]rgba, blockW*blockH)
	for by := 0; by < blocksY; by++ {
		for bx := 0; bx < blocksX; bx++ {
			off := (by*blocksX + bx) * astcBlockSize
			decodeASTCBlock(data[off:off+astcBlockSize], blockW, blockH, texels)
			for ty := 0; ty < blockH; ty++ {
				y := by*blockH + ty
				if y >= height {
					break
				}
				for tx := 0; tx < blockW; tx++ {
					x := bx*blockW + tx
					if x >= width {
						break
					}
					c := texels[ty*blockW+tx]
					i := img.PixOffset(x, y)
					img.Pix[i+0] = uint8(c.r)
					img.Pix[i+1] = uint8(c.g)
					img.Pix[i+2] = uint8(c.b)
					img.Pix[i+3] = uint8(c.a)
				}
			}
		}
	}
	return img, nil
}

// errorColour fills a block that cannot be decoded. ASTC hardware produces
// magenta for an undecodable block, and matching that makes a decode failure
// obvious in a preview rather than silently plausible.
var errorColour = rgba{255, 0, 255, 255}

func fill(texels []rgba, c rgba) {
	for i := range texels {
		texels[i] = c
	}
}

func decodeASTCBlock(block []byte, blockW, blockH int, texels []rgba) {
	var lo uint64
	for i := 0; i < 8; i++ {
		lo |= uint64(block[i]) << (8 * uint(i))
	}

	// Void extent: the whole block is one colour.
	if lo&0x1FF == 0x1FC {
		if lo&0x200 != 0 { // HDR void extent
			fill(texels, errorColour)
			return
		}
		rd := &bitReader{data: block, pos: 64}
		r := int(rd.read(16) >> 8)
		g := int(rd.read(16) >> 8)
		b := int(rd.read(16) >> 8)
		a := int(rd.read(16) >> 8)
		fill(texels, rgba{r, g, b, a})
		return
	}

	bm, ok := decodeBlockMode(uint32(lo & 0x7FF))
	if !ok || bm.weightsX > blockW || bm.weightsY > blockH {
		fill(texels, errorColour)
		return
	}

	partitions := int((lo>>11)&3) + 1
	if bm.dualPlane && partitions == 4 {
		fill(texels, errorColour) // not a legal combination
		return
	}

	dualBits := 0
	if bm.dualPlane {
		dualBits = 2
	}

	// Endpoint modes. A single-partition block states its mode in four bits at
	// 13. A multi-partition block packs one mode per partition into a field
	// that straddles the block: six bits at 23, and the rest immediately below
	// the weight data.
	cems := make([]int, partitions)
	endpointStart := 17 // 11 mode + 2 partition + 4 CEM
	highpartBits := 0
	belowWeights := 128 - bm.weightBits
	if partitions == 1 {
		cems[0] = int((lo >> 13) & 0xF)
	} else {
		highpartBits = 3*partitions - 4
		belowWeights -= highpartBits
		r := &bitReader{data: block, pos: 23}
		encoded := int(r.read(6))
		hp := &bitReader{data: block, pos: belowWeights}
		encoded |= int(hp.read(highpartBits)) << 6

		if baseClass := encoded & 3; baseClass == 0 {
			for i := range cems {
				cems[i] = (encoded >> 2) & 0xF
			}
			belowWeights += highpartBits
			highpartBits = 0
		} else {
			baseClass--
			bitpos := 2
			for i := 0; i < partitions; i++ {
				cems[i] = (((encoded >> uint(bitpos)) & 1) + baseClass) << 2
				bitpos++
			}
			for i := 0; i < partitions; i++ {
				cems[i] |= (encoded >> uint(bitpos)) & 3
				bitpos += 2
			}
		}
		endpointStart = 29 // 13 + 10 partition-index bits + 6 CEM bits
	}

	nEnd := 0
	for _, c := range cems {
		nEnd += endpointCount(c)
	}
	remaining := 128 - endpointStart - bm.weightBits - dualBits - highpartBits
	if remaining <= 0 || nEnd == 0 {
		fill(texels, errorColour)
		return
	}

	// The endpoint quantisation level is the largest one that still fits.
	colourLevels := 0
	for _, lv := range []int{256, 192, 160, 128, 96, 80, 64, 48, 40, 32, 24, 20, 16, 12, 10, 8, 6} {
		if iseBitCount(nEnd, lv) <= remaining {
			colourLevels = lv
			break
		}
	}
	tbl, haveTbl := colourUnquant[colourLevels]
	if !haveTbl {
		fill(texels, errorColour)
		return
	}

	rawEnd := decodeISE(block, endpointStart, nEnd, colourLevels)
	vals := make([]int, nEnd)
	for i, q := range rawEnd {
		if int(q) < len(tbl) {
			vals[i] = int(tbl[q])
		}
	}

	ep0 := make([]rgba, partitions)
	ep1 := make([]rgba, partitions)
	consumed := 0
	for i, c := range cems {
		n := endpointCount(c)
		a, b, ok := decodeEndpoints(c, vals[consumed:consumed+n])
		if !ok {
			fill(texels, errorColour)
			return
		}
		ep0[i], ep1[i] = a, b
		consumed += n
	}

	partitionSeed := 0
	if partitions > 1 {
		partitionSeed = int((lo >> 13) & 0x3FF)
	}

	// Weights live at the top of the block in reverse bit order.
	planes := 1
	if bm.dualPlane {
		planes = 2
	}
	rev := reverseBits128(block)
	levels := weightLevels[bm.quantMode]
	rawW := decodeISE(rev, 0, bm.weightsX*bm.weightsY*planes, levels)
	wtbl := weightUnquant[levels]
	grid := make([]int, len(rawW))
	for i, q := range rawW {
		if int(q) < len(wtbl) {
			grid[i] = int(wtbl[q])
		}
	}

	ccs := -1
	if bm.dualPlane {
		// The dual-plane channel selector sits just below the weight data, and
		// below the multi-partition mode bits when those are present.
		rd := &bitReader{data: block, pos: belowWeights - 2}
		ccs = int(rd.read(2))
	}

	smallBlock := blockW*blockH < 32

	// LDR interpolation: endpoints expand to 16 bits by replication, the
	// interpolation happens there, and the high byte is the result.
	interp := func(a, b, w int) int {
		c0, c1 := a<<8|a, b<<8|b
		return ((c0*(64-w) + c1*w + 32) >> 6) >> 8
	}

	for ty := 0; ty < blockH; ty++ {
		for tx := 0; tx < blockW; tx++ {
			p := 0
			if partitions > 1 {
				p = selectPartition(partitionSeed, tx, ty, partitions, smallBlock)
				if p >= partitions {
					p = partitions - 1
				}
			}
			e0, e1 := ep0[p], ep1[p]

			w0 := infillWeight(grid, bm.weightsX, bm.weightsY, blockW, blockH, tx, ty, planes, 0)
			wr, wg, wb, wa := w0, w0, w0, w0
			if bm.dualPlane {
				w1 := infillWeight(grid, bm.weightsX, bm.weightsY, blockW, blockH, tx, ty, planes, 1)
				switch ccs {
				case 0:
					wr = w1
				case 1:
					wg = w1
				case 2:
					wb = w1
				case 3:
					wa = w1
				}
			}
			texels[ty*blockW+tx] = rgba{
				interp(e0.r, e1.r, wr),
				interp(e0.g, e1.g, wg),
				interp(e0.b, e1.b, wb),
				interp(e0.a, e1.a, wa),
			}
		}
	}
}

// infillWeight resamples the block's weight grid at a texel position. The
// weight grid is usually coarser than the block, so the specification defines a
// fixed bilinear filter working in sixteenths; this is that filter.
func infillWeight(grid []int, wx, wy, blockW, blockH, tx, ty, planes, plane int) int {
	ds, dt := 0, 0
	if blockW > 1 {
		ds = (1024 + blockW/2) / (blockW - 1)
	}
	if blockH > 1 {
		dt = (1024 + blockH/2) / (blockH - 1)
	}
	gs := (ds*tx*(wx-1) + 32) >> 6
	gt := (dt*ty*(wy-1) + 32) >> 6

	js, fs := gs>>4, gs&0xF
	jt, ft := gt>>4, gt&0xF

	w11 := (fs*ft + 8) >> 4
	w10 := ft - w11
	w01 := fs - w11
	w00 := 16 - fs - ft + w11

	at := func(x, y int) int {
		if x >= wx {
			x = wx - 1
		}
		if y >= wy {
			y = wy - 1
		}
		if x < 0 || y < 0 {
			return 0
		}
		i := (y*wx+x)*planes + plane
		if i < 0 || i >= len(grid) {
			return 0
		}
		return grid[i]
	}
	return (at(js, jt)*w00 + at(js+1, jt)*w01 + at(js, jt+1)*w10 + at(js+1, jt+1)*w11 + 8) >> 4
}
