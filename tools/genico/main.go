package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"os"
)

var sizes = []int{16, 24, 32, 48, 64, 128, 256}

func main() {
	if len(os.Args) < 3 {
		log.Fatal("usage: genico <src.png> <out.ico>")
	}
	srcPath, outPath := os.Args[1], os.Args[2]

	f, err := os.Open(srcPath)
	if err != nil {
		log.Fatal(err)
	}
	src, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		log.Fatal(err)
	}

	// Не увеличиваем изображение выше исходного размера — только уменьшаем.
	b := src.Bounds()
	srcMax := b.Dx()
	if b.Dy() > srcMax {
		srcMax = b.Dy()
	}
	var selected []int
	for _, n := range sizes {
		if n <= srcMax {
			selected = append(selected, n)
		}
	}
	if len(selected) == 0 {
		selected = []int{sizes[0]}
	}

	var images [][]byte
	for _, n := range selected {
		img := renderSquare(src, n)
		images = append(images, encodeBMPIcon(img))
	}

	out := buildICO(images, selected)
	if err := os.WriteFile(outPath, out, 0644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s (%d entries, sizes %v)", outPath, len(images), selected)
}

// renderSquare fits src into an n×n transparent canvas preserving aspect ratio.
func renderSquare(src image.Image, n int) *image.NRGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()

	// contain fit
	fitW, fitH := sw, sh
	if sw*n > sh*n { // wider than tall relative to square → width-bound
		fitW = n
		fitH = sh * n / sw
	} else {
		fitH = n
		fitW = sw * n / sh
	}
	if fitW < 1 {
		fitW = 1
	}
	if fitH < 1 {
		fitH = 1
	}

	scaled := boxDownscale(src, fitW, fitH)

	canvas := image.NewNRGBA(image.Rect(0, 0, n, n))
	offX := (n - fitW) / 2
	offY := (n - fitH) / 2
	for y := 0; y < fitH; y++ {
		for x := 0; x < fitW; x++ {
			canvas.Set(offX+x, offY+y, scaled.At(x, y))
		}
	}
	return canvas
}

// boxDownscale averages source pixels into each destination pixel.
func boxDownscale(src image.Image, dw, dh int) *image.NRGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for dy := 0; dy < dh; dy++ {
		sy0 := b.Min.Y + dy*sh/dh
		sy1 := b.Min.Y + (dy+1)*sh/dh
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for dx := 0; dx < dw; dx++ {
			sx0 := b.Min.X + dx*sw/dw
			sx1 := b.Min.X + (dx+1)*sw/dw
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			var rs, gs, bs, as, cnt uint64
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					r, g, bl, a := src.At(sx, sy).RGBA()
					rs += uint64(r >> 8)
					gs += uint64(g >> 8)
					bs += uint64(bl >> 8)
					as += uint64(a >> 8)
					cnt++
				}
			}
			if cnt == 0 {
				cnt = 1
			}
			dst.Set(dx, dy, color.NRGBA{
				R: uint8(rs / cnt),
				G: uint8(gs / cnt),
				B: uint8(bs / cnt),
				A: uint8(as / cnt),
			})
		}
	}
	return dst
}

// encodeBMPIcon encodes an image as a 32bpp BMP icon image (DIB + AND mask).
func encodeBMPIcon(img *image.NRGBA) []byte {
	n := img.Bounds().Dx()
	var buf bytes.Buffer

	// BITMAPINFOHEADER
	binary.Write(&buf, binary.LittleEndian, uint32(40)) // biSize
	binary.Write(&buf, binary.LittleEndian, int32(n))   // biWidth
	binary.Write(&buf, binary.LittleEndian, int32(2*n)) // biHeight (color + mask)
	binary.Write(&buf, binary.LittleEndian, uint16(1))  // biPlanes
	binary.Write(&buf, binary.LittleEndian, uint16(32)) // biBitCount
	binary.Write(&buf, binary.LittleEndian, uint32(0))  // biCompression BI_RGB
	binary.Write(&buf, binary.LittleEndian, uint32(0))  // biSizeImage
	binary.Write(&buf, binary.LittleEndian, int32(0))   // biXPelsPerMeter
	binary.Write(&buf, binary.LittleEndian, int32(0))   // biYPelsPerMeter
	binary.Write(&buf, binary.LittleEndian, uint32(0))  // biClrUsed
	binary.Write(&buf, binary.LittleEndian, uint32(0))  // biClrImportant

	// XOR color data: bottom-up, BGRA
	for y := n - 1; y >= 0; y-- {
		for x := 0; x < n; x++ {
			c := img.NRGBAAt(x, y)
			buf.WriteByte(c.B)
			buf.WriteByte(c.G)
			buf.WriteByte(c.R)
			buf.WriteByte(c.A)
		}
	}

	// AND mask: 1bpp, bottom-up, rows padded to 4 bytes, all zero (use alpha).
	maskRow := ((n + 31) / 32) * 4
	mask := make([]byte, maskRow*n)
	buf.Write(mask)

	return buf.Bytes()
}

// buildICO assembles ICONDIR + entries + image data.
func buildICO(images [][]byte, dims []int) []byte {
	var buf bytes.Buffer

	// ICONDIR
	binary.Write(&buf, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // type = icon
	binary.Write(&buf, binary.LittleEndian, uint16(len(images)))

	offset := 6 + 16*len(images)
	for i, data := range images {
		n := dims[i]
		wb := byte(n)
		hb := byte(n)
		if n >= 256 {
			wb, hb = 0, 0
		}
		buf.WriteByte(wb)                                           // width
		buf.WriteByte(hb)                                           // height
		buf.WriteByte(0)                                            // color count
		buf.WriteByte(0)                                            // reserved
		binary.Write(&buf, binary.LittleEndian, uint16(1))         // planes
		binary.Write(&buf, binary.LittleEndian, uint16(32))        // bitcount
		binary.Write(&buf, binary.LittleEndian, uint32(len(data))) // bytesInRes
		binary.Write(&buf, binary.LittleEndian, uint32(offset))    // imageOffset
		offset += len(data)
	}
	for _, data := range images {
		buf.Write(data)
	}
	return buf.Bytes()
}
