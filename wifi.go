package main

import (
	_ "embed"
	"log"

	"github.com/rodrigocfd/windigo/win"
)

//go:embed assets/Wi-Fi_QR_code_ARTES-GUEST.jpg
var wifiQRImage []byte

// appWifi — декодированная картинка QR-кода подключения к Wi-Fi (показывается
// по эндпойнту /wifi). nil, если картинку не удалось декодировать.
var appWifi *logoSource

// initAppWifi декодирует встроенную картинку Wi-Fi один раз при старте.
func initAppWifi() {
	src, err := decodeLogoImage(wifiQRImage)
	if err != nil {
		log.Printf("Не удалось декодировать картинку Wi-Fi: %v", err)
		return
	}
	appWifi = src
}

// drawWifiImage рисует картинку Wi-Fi по центру всей области rc с сохранением
// пропорций (режим /wifi).
func drawWifiImage(hdc win.HDC, rc win.RECT) {
	ensureBrushes()
	fillRow(hdc, rc.Left, rc.Top, rc.Right, rc.Bottom, brBg)

	if appWifi == nil {
		return
	}

	w := rc.Right - rc.Left
	h := rc.Bottom - rc.Top
	iw, ih := rectContain(appWifi.width, appWifi.height, w*90/100, h*90/100)

	scaled := scaleLogoOverWhite(appWifi.img, int(iw), int(ih))
	x := rc.Left + (w-iw)/2
	y := rc.Top + (h-ih)/2
	blitImage(hdc, x, y, scaled)
}

// rectContain вписывает изображение sw×sh в прямоугольник boxW×boxH с
// сохранением пропорций.
func rectContain(sw, sh, boxW, boxH int32) (int32, int32) {
	if sw < 1 || sh < 1 || boxW < 1 || boxH < 1 {
		return boxW, boxH
	}
	if sw*boxH > boxW*sh { // ограничивает ширина
		h := sh * boxW / sw
		if h < 1 {
			h = 1
		}
		return boxW, h
	}
	// ограничивает высота
	w := sw * boxH / sh
	if w < 1 {
		w = 1
	}
	return w, boxH
}
