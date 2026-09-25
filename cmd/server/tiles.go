package main

import (
	"image"
	"image/color"
	"image/draw"
	"math/rand"
)

const tileSize = 120
const wallThick = 30

var (
	wallBase  = color.RGBA{0xE8, 0xBE, 0x86, 0xFF}
	wallLight = color.RGBA{0xF2, 0xCE, 0x9A, 0xFF}
	wallDark  = color.RGBA{0xCC, 0x9C, 0x62, 0xFF}
	outline   = color.RGBA{0x2A, 0x1A, 0x0E, 0xFF}

	floorBase  = color.RGBA{0x40, 0x32, 0x26, 0xFF}
	floorLight = color.RGBA{0x50, 0x40, 0x30, 0xFF}
	floorDark  = color.RGBA{0x30, 0x24, 0x1A, 0xFF}
)

func generateTile(openN, openS, openE, openW bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, tileSize, tileSize))

	drawFloor(img, 0, 0, tileSize, tileSize)

	if !openN {
		drawWallHSeg(img, 0, 0, tileSize, wallThick)
	}
	if !openS {
		drawWallHSeg(img, 0, tileSize-wallThick, tileSize, wallThick)
	}
	if !openW {
		drawWallVSeg(img, 0, 0, wallThick, tileSize)
	}
	if !openE {
		drawWallVSeg(img, tileSize-wallThick, 0, wallThick, tileSize)
	}

	drawCorner(img, 0, 0)
	drawCorner(img, tileSize-wallThick, 0)
	drawCorner(img, 0, tileSize-wallThick)
	drawCorner(img, tileSize-wallThick, tileSize-wallThick)

	return img
}

func drawFloor(img *image.RGBA, ox, oy, w, h int) {
	fillRect(img, ox, oy, w, h, floorBase)

	for y := oy + 6; y < oy+h-6; y += 20 {
		for x := ox + 6; x < ox+w-6; x += 22 {
			offX := rand.Intn(4) - 2
			offY := rand.Intn(4) - 2
			rw := 14 + rand.Intn(8)
			rh := 12 + rand.Intn(6)
			if x+offX+rw > ox+w-2 {
				rw = ox + w - 2 - x - offX
			}
			if y+offY+rh > oy+h-2 {
				rh = oy + h - 2 - y - offY
			}
			c := floorLight
			if rand.Intn(3) == 0 {
				c = floorDark
			}
			fillRoundedRect(img, x+offX, y+offY, rw, rh, 3, c)
		}
	}

	for y := oy; y < oy+h; y++ {
		for x := ox; x < ox+w; x++ {
			r32, g32, b32, _ := img.At(x, y).RGBA()
			rr := uint8(r32 >> 8)
			gg := uint8(g32 >> 8)
			bb := uint8(b32 >> 8)
			if rand.Intn(100) < 6 {
				rr = uint8(uint32(rr)*3/4 + 20)
				gg = uint8(uint32(gg)*3/4 + 15)
				bb = uint8(uint32(bb)*3/4 + 10)
			}
			img.Set(x, y, color.RGBA{rr, gg, bb, 0xFF})
		}
	}
}

func drawWallHSeg(img *image.RGBA, ox, oy, w, h int) {
	if w == 0 || h == 0 {
		return
	}

	brickRows := []int{}
	for row := 0; row < h; {
		bh := 8 + rand.Intn(4)
		if row+bh > h {
			bh = h - row
		}
		brickRows = append(brickRows, bh)
		row += bh
	}

	var brickData []struct{ bx, by, bw int }
	rowY := 0
	for rowIdx, bh := range brickRows {
		col := 0
		stagger := 0
		if rowIdx%2 == 1 {
			stagger = 7
		}
		for col < w {
			bw := 14 + rand.Intn(10)
			if col+bw > w {
				bw = w - col
			}

			if rowY > 0 {
				aboveBrick := findBrickAt(brickData, col+stagger/2, rowY)
				if aboveBrick >= 0 && brickData[aboveBrick].bx+brickData[aboveBrick].bw > col {
					col = brickData[aboveBrick].bx + brickData[aboveBrick].bw - 2
					continue
				}
			}

			brickData = append(brickData, struct{ bx, by, bw int }{col + stagger/3, oy + rowY, bw - 1})
			col += bw
		}
		rowY += bh
	}

	fillRect(img, ox, oy, w, h, wallBase)

	for _, b := range brickData {
		bx, by := ox+b.bx, oy+b.by
		if bx < ox {
			bx = ox
		}
		if by < oy {
			by = oy
		}
		be := bx + b.bw
		if be > ox+w {
			be = ox + w
		}
		bw := be - bx
		be2 := by + bhFromBricks(brickRows, b.by-oy)
		if be2 > oy+h {
			be2 = oy + h
		}
		bh := be2 - by

		if bw <= 0 || bh <= 0 {
			continue
		}

		n := rand.Intn(4)
		c := wallBase
		if n == 0 {
			c = wallLight
		} else if n == 1 {
			c = wallDark
		}
		offX := rand.Intn(2)
		offY := rand.Intn(2)
		fillRoundedRect(img, bx+offX, by+offY, bw, bh, 2, c)

	}

	for y := oy; y < oy+h; y++ {
		for x := ox; x < ox+w; x++ {
			r32, g32, b32, _ := img.At(x, y).RGBA()
			rr, gg, bb := uint8(r32>>8), uint8(g32>>8), uint8(b32>>8)
			if rand.Intn(100) < 4 {
				rr = uint8(uint32(rr)*4/5 + 20)
				gg = uint8(uint32(gg)*4/5 + 16)
				bb = uint8(uint32(bb)*4/5 + 12)
			}
			img.Set(x, y, color.RGBA{rr, gg, bb, 0xFF})
		}
	}

	outlineRect(img, ox, oy, w, h, outline)
}

func findBrickAt(data []struct{ bx, by, bw int }, x, y int) int {
	for i, b := range data {
		if b.by == y && x >= b.bx && x < b.bx+b.bw {
			return i
		}
	}
	return -1
}

func bhFromBricks(rows []int, target int) int {
	acc := 0
	for _, bh := range rows {
		acc += bh
		if acc > target {
			return bh
		}
	}
	if len(rows) > 0 {
		return rows[len(rows)-1]
	}
	return 10
}

func drawWallVSeg(img *image.RGBA, ox, oy, w, h int) {
	if w == 0 || h == 0 {
		return
	}
	drawWallHSeg(img, ox, oy, w, h)
}

func drawCorner(img *image.RGBA, ox, oy int) {
	fillRoundedRect(img, ox+1, oy+1, wallThick-2, wallThick-2, 4, wallLight)
	fillRoundedRect(img, ox+2, oy+2, wallThick-4, wallThick-4, 3, wallBase)

	n := rand.Intn(3)
	c := wallLight
	if n == 0 {
		c = wallDark
	} else if n == 1 {
		c = wallBase
	}
	fillRoundedRect(img, ox+3, oy+3, wallThick-6, wallThick-6, 2, c)

	for i := 0; i < 4; i++ {
		sx, sy := ox+wallThick-1, oy+i*8+4
		if sy >= oy+wallThick {
			break
		}
		if rand.Intn(2) == 0 {
			img.Set(sx, sy, outline)
		}
		sx2, sy2 := ox+i*8+4, oy+wallThick-1
		if sx2 >= ox+wallThick {
			break
		}
		if rand.Intn(2) == 0 {
			img.Set(sx2, sy2, outline)
		}
	}

	outlineRect(img, ox, oy, wallThick, wallThick, outline)
}

func fillRect(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	draw.Draw(img, image.Rect(x, y, x+w, y+h), image.NewUniform(c), image.Point{}, draw.Over)
}

func fillRoundedRect(img *image.RGBA, x, y, w, h, r int, c color.RGBA) {
	for py := y; py < y+h && py < tileSize; py++ {
		if py < 0 {
			continue
		}
		for px := x; px < x+w && px < tileSize; px++ {
			if px < 0 {
				continue
			}
			inCorner := false
			if px < x+r && py < y+r {
				dx, dy := px-(x+r-1), py-(y+r-1)
				inCorner = dx*dx+dy*dy > r*r-1
			} else if px >= x+w-r && py < y+r {
				dx, dy := px-(x+w-r), py-(y+r-1)
				inCorner = dx*dx+dy*dy > r*r-1
			} else if px < x+r && py >= y+h-r {
				dx, dy := px-(x+r-1), py-(y+h-r)
				inCorner = dx*dx+dy*dy > r*r-1
			} else if px >= x+w-r && py >= y+h-r {
				dx, dy := px-(x+w-r), py-(y+h-r)
				inCorner = dx*dx+dy*dy > r*r-1
			}
			if !inCorner {
				img.Set(px, py, c)
			}
		}
	}
}

func outlineRect(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	for px := x; px < x+w && px < tileSize; px++ {
		if px < 0 {
			continue
		}
		img.Set(px, y, c)
		if y+h-1 < tileSize {
			img.Set(px, y+h-1, c)
		}
	}
	for py := y; py < y+h && py < tileSize; py++ {
		if py < 0 {
			continue
		}
		img.Set(x, py, c)
		if x+w-1 < tileSize {
			img.Set(x+w-1, py, c)
		}
	}
}
