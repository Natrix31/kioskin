package main

import (
	_ "embed"
	"image"
	"image/color"
	"log"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/win"
	"github.com/skip2/go-qrcode"
)

//go:embed assets/TelegramLogo.png
var telegramLogoPNG []byte

//go:embed assets/MAX-256x256.png
var maxLogoPNG []byte

// qrEntry — один QR-код с подписью и логотипом приложения под ним.
type qrEntry struct {
	code    *qrcode.QRCode
	caption string
	logo    *logoSource // логотип рядом с подписью; nil — только подпись
}

// appQRCodes — QR-коды, показываемые в нижней части основного окна. Заполняется
// один раз при старте из config.json (ссылки на ботов). Пустой срез — QR не
// показываются, список покупок занимает всю высоту.
var appQRCodes []qrEntry

// initAppQRCodes готовит QR-коды из ссылок в конфиге. Пустая ссылка — QR
// пропускается. Ошибка кодирования не фатальна: просто без этого QR.
func initAppQRCodes(cfg appConfig) {
	appQRCodes = appQRCodes[:0]
	add := func(url, caption string, logoPNG []byte) {
		if url == "" {
			return
		}
		code, err := qrcode.New(url, qrcode.Medium)
		if err != nil {
			log.Printf("Не удалось сгенерировать QR для %s (%s): %v", caption, url, err)
			return
		}
		entry := qrEntry{code: code, caption: caption}
		if logo, err := decodeLogoImage(logoPNG); err != nil {
			log.Printf("Не удалось декодировать логотип %s: %v", caption, err)
		} else {
			entry.logo = logo
		}
		appQRCodes = append(appQRCodes, entry)
	}
	add(cfg.TelegramURL, "Telegram", telegramLogoPNG)
	add(cfg.MaxURL, "MAX", maxLogoPNG)
}

// Геометрия полосы QR (px при 96 DPI).
const (
	qrPad        = 18  // отступ вокруг QR внутри ячейки
	qrCaptionH   = 40  // высота строки подписи (≈ высота шрифта списка + запас)
	qrMaxSize    = 185 // базовый размер QR в полосе под списком (при 1920x1080)
	qrMinSize    = 64  // минимальный размер QR
	qrSocialsMax = 185 // базовый размер QR в /socials при 1920x1080 (коды столбиком)
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
	if qrCap := scalePx(qrMaxSize); qr > qrCap {
		qr = qrCap
	}
	if qr < qrMinSize {
		qr = qrMinSize
	}
	band = qr + qrCaptionH + 3*qrPad
	return band, qr
}

// drawWindowContent рисует содержимое основного окна: список покупок сверху и,
// если заданы ссылки и showQR=true (режим списка/пречека), полосу QR-кодов внизу.
// В стартовом полупрозрачном состоянии showQR=false — QR не показываются.
func drawWindowContent(hdc win.HDC, rc win.RECT, items []purchaseItem, showQR bool) {
	if !showQR || len(appQRCodes) == 0 {
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
	if len(entries) == 0 {
		return
	}
	ensureBrushes()

	band, qr := qrMetrics(rc.Right-rc.Left, rc.Bottom-rc.Top, int32(len(entries)))
	top := rc.Bottom - band

	// Фон полосы (непрозрачный, в тон списка).
	fillRow(hdc, rc.Left, top, rc.Right, rc.Bottom, brBg)
	drawQRRow(hdc, rc.Left, rc.Right, top+qrPad, qr, entries)
}

// drawSocialsQRCodes рисует QR-коды по центру экрана столбиком (один под
// другим) — режим /socials, без списка покупок.
func drawSocialsQRCodes(hdc win.HDC, rc win.RECT) {
	ensureBrushes()

	// Непрозрачный фон на весь экран.
	fillRow(hdc, rc.Left, rc.Top, rc.Right, rc.Bottom, brBg)

	entries := appQRCodes
	n := int32(len(entries))
	if n == 0 {
		return
	}

	w := rc.Right - rc.Left
	h := rc.Bottom - rc.Top

	// Экран делится по вертикали на n равных полос (для двух кодов — пополам);
	// в каждой полосе QR-блок центрируется.
	segH := h / n

	qr := scalePx(qrSocialsMax)
	if maxByW := w - 2*qrPad; qr > maxByW {
		qr = maxByW
	}
	if maxByH := segH - qrCaptionH - qrPad; qr > maxByH {
		qr = maxByH
	}
	if qr < qrMinSize {
		qr = qrMinSize
	}

	textH := setupQRText(hdc)
	blockH := qr + qrPad/2 + textH // высота блока «QR + подпись»
	cx := rc.Left + w/2
	for i, e := range entries {
		segTop := rc.Top + segH*int32(i)
		top := segTop + (segH-blockH)/2
		if top < segTop {
			top = segTop
		}
		drawQREntry(hdc, cx, top, qr, textH, e)
	}
}

// drawQRRow рисует entries в один ряд: каждый QR стороной qr по центру своей
// ячейки, верхний край на y = top, под ним — подпись.
func drawQRRow(hdc win.HDC, left, right, top, qr int32, entries []qrEntry) {
	n := int32(len(entries))
	if n == 0 {
		return
	}
	textH := setupQRText(hdc)
	cellW := (right - left) / n
	for i, e := range entries {
		cx := left + cellW*int32(i) + cellW/2
		drawQREntry(hdc, cx, top, qr, textH, e)
	}
}

// setupQRText выбирает шрифт подписей, прозрачный фон и цвет текста; возвращает
// высоту строки текста.
func setupQRText(hdc win.HDC) int32 {
	if f := ensureListFont(); f != 0 {
		hdc.SelectObjectFont(f)
	}
	hdc.SetBkMode(co.BKMODE_TRANSPARENT)
	hdc.SetTextColor(clrText)
	textH := int32(28)
	if tm, err := hdc.GetTextMetrics(); err == nil {
		textH = int32(tm.Height)
	}
	return textH
}

// drawQREntry рисует один QR стороной qr (верхний край на y, центр по
// горизонтали в cx) и под ним подпись «[логотип] Название». Шрифт/цвет должны
// быть уже настроены через setupQRText.
func drawQREntry(hdc win.HDC, cx, top, qr, textH int32, e qrEntry) {
	img := e.code.Image(int(qr))
	aw := int32(img.Bounds().Dx())
	ah := int32(img.Bounds().Dy())
	blitImage(hdc, cx-aw/2, top+(qr-ah)/2, img)

	capY := top + qr + qrPad/2
	tw := textWidth(hdc, e.caption)

	const logoGap = 10
	var (
		logoImg image.Image
		logoW   int32
		logoH   int32
	)
	if e.logo != nil {
		logoW, logoH = containSize(e.logo.width, e.logo.height, textH*3/2)
		logoImg = scaleLogoOverWhite(e.logo.img, int(logoW), int(logoH))
	}

	total := tw
	if logoImg != nil {
		total += logoW + logoGap
	}
	x := cx - total/2
	if logoImg != nil {
		blitImage(hdc, x, capY+textH/2-logoH/2, logoImg)
		x += logoW + logoGap
	}
	hdc.TextOut(int(x), int(capY), e.caption)
}

// containSize вписывает изображение sw×sh в квадрат box×box с сохранением
// пропорций.
func containSize(sw, sh, box int32) (int32, int32) {
	if sw < 1 || sh < 1 {
		return box, box
	}
	if sw >= sh {
		h := box * sh / sw
		if h < 1 {
			h = 1
		}
		return box, h
	}
	w := box * sw / sh
	if w < 1 {
		w = 1
	}
	return w, box
}

// scaleLogoOverWhite масштабирует src в dstW×dstH усреднением пикселей и
// накладывает результат на белый фон (для полупрозрачных PNG), возвращая
// непрозрачное изображение — готовое для blitImage.
func scaleLogoOverWhite(src image.Image, dstW, dstH int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw < 1 || sh < 1 || dstW < 1 || dstH < 1 {
		return dst
	}

	for dy := 0; dy < dstH; dy++ {
		sy0 := b.Min.Y + dy*sh/dstH
		sy1 := b.Min.Y + (dy+1)*sh/dstH
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for dx := 0; dx < dstW; dx++ {
			sx0 := b.Min.X + dx*sw/dstW
			sx1 := b.Min.X + (dx+1)*sw/dstW
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			var rs, gs, bs, as, cnt uint64
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					r, g, bl, a := src.At(sx, sy).RGBA() // премультиплиц., 16 бит
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
			// Наложение на белый: out = premult + 255*(1 - alpha).
			whiteAdd := 255 - as/cnt
			dst.SetRGBA(dx, dy, color.RGBA{
				R: clamp8(rs/cnt + whiteAdd),
				G: clamp8(gs/cnt + whiteAdd),
				B: clamp8(bs/cnt + whiteAdd),
				A: 255,
			})
		}
	}
	return dst
}

func clamp8(v uint64) uint8 {
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// blitImage выводит изображение img на hdc в позицию (x, y) без масштабирования
// (1:1). Изображение переносится в DIB-секцию (imageToBitmap) и копируется через
// memory-DC + BitBlt — этот путь надёжно работает и для крупных изображений
// (в отличие от StretchDIBits, который на больших блитах не срабатывал).
func blitImage(hdc win.HDC, x, y int32, img image.Image) {
	b := img.Bounds()
	w := int32(b.Dx())
	h := int32(b.Dy())
	if w < 1 || h < 1 {
		return
	}

	hBmp, err := imageToBitmap(img)
	if err != nil {
		log.Printf("Не удалось подготовить изображение: %v", err)
		return
	}
	defer hBmp.DeleteObject()

	memDC, err := hdc.CreateCompatibleDC()
	if err != nil {
		log.Printf("Не удалось создать memory DC: %v", err)
		return
	}
	defer memDC.DeleteDC()

	prev, err := memDC.SelectObjectBmp(hBmp)
	if err != nil {
		log.Printf("Не удалось выбрать bitmap в memory DC: %v", err)
		return
	}
	defer memDC.SelectObjectBmp(prev)

	if err := hdc.BitBlt(
		win.POINT{X: x, Y: y}, win.SIZE{Cx: w, Cy: h},
		memDC, win.POINT{X: 0, Y: 0}, co.ROP_SRCCOPY,
	); err != nil {
		log.Printf("Не удалось нарисовать изображение: %v", err)
	}
}
