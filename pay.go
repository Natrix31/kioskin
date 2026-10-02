package main

import (
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"github.com/rodrigocfd/windigo/win"
	"github.com/skip2/go-qrcode"
)

// appPayQR — QR-код платёжной ссылки, присланной в POST /pay. Генерируется на
// каждый запрос; защищён payMu (пишется HTTP-горутиной, читается UI-потоком).
var (
	payMu    sync.RWMutex
	appPayQR *qrcode.QRCode
)

// decodeBase64Link разбирает base64 (std/url, с паддингом и без) из тела запроса
// и возвращает исходную строку-ссылку.
func decodeBase64Link(body []byte) (string, error) {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return "", fmt.Errorf("пустое тело запроса")
	}
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	} {
		if dec, err := enc.DecodeString(s); err == nil {
			link := strings.TrimSpace(string(dec))
			if link == "" {
				return "", fmt.Errorf("пустая ссылка после декодирования")
			}
			return link, nil
		}
	}
	return "", fmt.Errorf("некорректный base64")
}

// setPayQR генерирует QR из ссылки и атомарно сохраняет его как текущий
// платёжный код. Ошибка — если ссылка слишком длинная для QR.
func setPayQR(link string) error {
	code, err := qrcode.New(link, qrcode.Medium)
	if err != nil {
		return err
	}
	payMu.Lock()
	appPayQR = code
	payMu.Unlock()
	return nil
}

// activePayQR возвращает текущий платёжный QR (или nil).
func activePayQR() *qrcode.QRCode {
	payMu.RLock()
	defer payMu.RUnlock()
	return appPayQR
}

// drawPayQR рисует платёжный QR крупно по центру всей области rc (режим /pay).
func drawPayQR(hdc win.HDC, rc win.RECT) {
	ensureBrushes()
	fillRow(hdc, rc.Left, rc.Top, rc.Right, rc.Bottom, brBg)

	code := activePayQR()
	if code == nil {
		return
	}

	w := rc.Right - rc.Left
	h := rc.Bottom - rc.Top
	side := w
	if h < side {
		side = h
	}
	side = side * 80 / 100 // с полями

	img := code.Image(int(side))
	aw := int32(img.Bounds().Dx())
	ah := int32(img.Bounds().Dy())
	x := rc.Left + (w-aw)/2
	y := rc.Top + (h-ah)/2
	blitImage(hdc, x, y, img)
}
