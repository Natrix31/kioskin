package main

import (
	"image"
	"log"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/win"
	"github.com/skip2/go-qrcode"
)

// qrEntry — один QR-код с подписью под ним.
type qrEntry struct {
	code    *qrcode.QRCode
	caption string
}

// appQRCodes — QR-коды, показываемые в нижней части основного окна. Заполняется
// один раз при старте из config.json (ссылки на ботов). Пустой срез — QR не
// показываются, список покупок занимает всю высоту.
var appQRCodes []qrEntry

// initAppQRCodes готовит QR-коды из ссылок в конфиге. Пустая ссылка — QR
// пропускается. Ошибка кодирования не фатальна: просто без этого QR.
func initAppQRCodes(cfg appConfig) {
	appQRCodes = appQRCodes[:0]
	add := func(url, caption string) {
		if url == "" {
			return
		}
		code, err := qrcode.New(url, qrcode.Medium)
		if err != nil {
			log.Printf("Не удалось сгенерировать QR для %s (%s): %v", caption, url, err)
			return
		}
		appQRCodes = append(appQRCodes, qrEntry{code: code, caption: caption})
	}
	add(cfg.TelegramURL, "Telegram")
	add(cfg.MaxURL, "MAX")
}

// Геометрия полосы QR (px при 96 DPI).
const (
	qrPad      = 18  // отступ вокруг QR внутри ячейки
	qrCaptionH = 40  // высота строки подписи (≈ высота шрифта списка + запас)
	qrMaxSize  = 420 // максимальный размер QR, чтобы не раздувать на 4K
	qrMinSize  = 64  // минимальный размер QR
)

// qrMetrics вычисляет высоту полосы QR и размер стороны одного QR-кода для
// области шириной w и высотой h, в которую нужно вписать n кодов в ряд.
func qrMetrics(w, h, n int32) (band, qr int32) {
	if n < 1 {
		n = 1
	}
	cellW := w / n
	qr = cellW - 2*qrPad
	if maxByH := h*42/100 - qrCaptionH - 3*qrPad; qr > maxByH {
		qr = maxByH
	}
	if qr > qrMaxSize {
		qr = qrMaxSize
	}
	if qr < qrMinSize {
		qr = qrMinSize
	}
	band = qr + qrCaptionH + 3*qrPad
	return band, qr
}

// drawWindowContent рисует содержимое основного окна: список покупок сверху и,
// если заданы ссылки, полосу QR-кодов внизу.
func drawWindowContent(hdc win.HDC, rc win.RECT, items []purchaseItem) {
	if len(appQRCodes) == 0 {
		drawPurchaseList(hdc, rc, items)
		return
	}

	band, _ := qrMetrics(rc.Right-rc.Left, rc.Bottom-rc.Top, int32(len(appQRCodes)))

	listRc := rc
	listRc.Bottom = rc.Bottom - band
	if listRc.Bottom < listRc.Top {
		listRc.Bottom = listRc.Top
	}
	drawPurchaseList(hdc, listRc, items)
	drawQRBand(hdc, rc, appQRCodes)
}

// drawQRBand рисует полосу QR-кодов внизу области rc: каждый код по центру своей
// ячейки, под ним — подпись. Полоса заливается фоном списка (непрозрачная).
func drawQRBand(hdc win.HDC, rc win.RECT, entries []qrEntry) {
	n := int32(len(entries))
	if n == 0 {
		return
	}
	ensureBrushes()

	band, qr := qrMetrics(rc.Right-rc.Left, rc.Bottom-rc.Top, n)
	top := rc.Bottom - band

	// Фон полосы (непрозрачный, в тон списка).
	fillRow(hdc, rc.Left, top, rc.Right, rc.Bottom, brBg)

	if f := ensureListFont(); f != 0 {
		hdc.SelectObjectFont(f)
	}
	hdc.SetBkMode(co.BKMODE_TRANSPARENT)
	hdc.SetTextColor(clrText)

	cellW := (rc.Right - rc.Left) / n
	for i, e := range entries {
		cx := rc.Left + cellW*int32(i) + cellW/2
		qy := top + qrPad

		img := e.code.Image(int(qr))
		aw := int32(img.Bounds().Dx())
		ah := int32(img.Bounds().Dy())
		ix := cx - aw/2
		iy := qy + (qr-ah)/2
		blitImage(hdc, ix, iy, img)

		tw := textWidth(hdc, e.caption)
		hdc.TextOut(int(cx-tw/2), int(qy+qr+qrPad/2), e.caption)
	}
}

// blitImage выводит изображение img на hdc в позицию (x, y) без масштабирования
// (1:1) через StretchDIBits — не создавая промежуточных HBITMAP.
func blitImage(hdc win.HDC, x, y int32, img image.Image) {
	b := img.Bounds()
	w := int32(b.Dx())
	h := int32(b.Dy())
	if w < 1 || h < 1 {
		return
	}

	stride := int(w) * 4
	bits := make([]byte, stride*int(h))
	// Bottom-up DIB: первая строка буфера — нижняя строка изображения.
	for iy := 0; iy < int(h); iy++ {
		dstRow := (int(h) - 1 - iy) * stride
		srcY := b.Min.Y + iy
		for ix := 0; ix < int(w); ix++ {
			r, g, bl, a := img.At(b.Min.X+ix, srcY).RGBA()
			o := dstRow + ix*4
			bits[o] = byte(bl >> 8)
			bits[o+1] = byte(g >> 8)
			bits[o+2] = byte(r >> 8)
			bits[o+3] = byte(a >> 8)
		}
	}

	var bi win.BITMAPINFO
	bi.BmiHeader.SetBiSize()
	bi.BmiHeader.Width = w
	bi.BmiHeader.Height = h // положительная высота — bottom-up
	bi.BmiHeader.Planes = 1
	bi.BmiHeader.BitCount = co.BITCOUNT_32
	bi.BmiHeader.Compression = co.BI_RGB

	if _, err := hdc.StretchDIBits(
		win.POINT{X: x, Y: y}, win.SIZE{Cx: w, Cy: h},
		win.POINT{X: 0, Y: 0}, win.SIZE{Cx: w, Cy: h},
		&bits[0], &bi, co.DIB_COLORS_RGB, co.ROP_SRCCOPY,
	); err != nil {
		log.Printf("Не удалось нарисовать QR: %v", err)
	}
}
