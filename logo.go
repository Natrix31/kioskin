package main

import (
	"bytes"
	_ "embed"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"unsafe"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"
)

//go:embed assets/logo.png
var defaultLogoPNG []byte

const (
	stmSetImage = 0x0172
	imageBitmap = 0
)

type logoSource struct {
	img    image.Image
	width  int32
	height int32
}

var appLogo *logoSource

func initAppLogo(cfg appConfig) error {
	data := defaultLogoPNG
	if cfg.LogoPath != "" {
		fileData, err := os.ReadFile(cfg.LogoPath)
		if err != nil {
			log.Printf("Логотип %s не прочитан (%v), используется встроенный", cfg.LogoPath, err)
		} else {
			data = fileData
		}
	}

	src, err := decodeLogoImage(data)
	if err != nil {
		return err
	}

	appLogo = src
	return nil
}

func decodeLogoImage(data []byte) (*logoSource, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width < 1 || height < 1 {
		return nil, os.ErrInvalid
	}

	return &logoSource{
		img:    img,
		width:  int32(width),
		height: int32(height),
	}, nil
}

func scaleImage(src image.Image, dstW, dstH int) image.Image {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW < 1 || srcH < 1 || dstW < 1 || dstH < 1 {
		return src
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		srcY := bounds.Min.Y + y*srcH/dstH
		for x := 0; x < dstW; x++ {
			srcX := bounds.Min.X + x*srcW/dstW
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

func imageToBitmap(img image.Image) (win.HBITMAP, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width < 1 || height < 1 {
		return 0, os.ErrInvalid
	}

	stride := width * 4
	pixels := make([]byte, stride*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, a := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			i := y*stride + x*4
			pixels[i] = byte(b >> 8)
			pixels[i+1] = byte(g >> 8)
			pixels[i+2] = byte(r >> 8)
			pixels[i+3] = byte(a >> 8)
		}
	}

	var bi win.BITMAPINFO
	bi.BmiHeader.SetBiSize()
	bi.BmiHeader.Width = int32(width)
	bi.BmiHeader.Height = -int32(height)
	bi.BmiHeader.Planes = 1
	bi.BmiHeader.BitCount = co.BITCOUNT_32
	bi.BmiHeader.Compression = co.BI_RGB

	hdcScreen, err := win.HWND(0).GetDC()
	if err != nil {
		return 0, err
	}
	defer win.HWND(0).ReleaseDC(hdcScreen)

	hBmp, bits, err := hdcScreen.CreateDIBSection(&bi, co.DIB_COLORS_RGB, 0, 0)
	if err != nil {
		return 0, err
	}

	copy(unsafe.Slice(bits, len(pixels)), pixels)
	return hBmp, nil
}

func applyLogoToStatic(logo *ui.Static, bmp win.HBITMAP) {
	if logo == nil || bmp == 0 {
		return
	}

	prev, err := logo.Hwnd().SendMessage(
		co.WM(stmSetImage),
		win.WPARAM(imageBitmap),
		win.LPARAM(uintptr(bmp)),
	)
	if err != nil {
		log.Printf("Не удалось установить логотип: %v", err)
		return
	}
	if prev != 0 {
		if err := win.HBITMAP(prev).DeleteObject(); err != nil {
			log.Printf("Не удалось удалить предыдущий bitmap логотипа: %v", err)
		}
	}
}

// applyImageAtSize scales src to width×height and sets it as the control bitmap.
func applyImageAtSize(ctrl *ui.Static, src *logoSource, width, height int32) {
	if ctrl == nil || src == nil || width < 1 || height < 1 {
		return
	}

	scaled := scaleImage(src.img, int(width), int(height))
	hBmp, err := imageToBitmap(scaled)
	if err != nil {
		log.Printf("Не удалось создать bitmap изображения: %v", err)
		return
	}

	applyLogoToStatic(ctrl, hBmp)
}

func applyLogoAtSize(logo *ui.Static, width, height int32) {
	applyImageAtSize(logo, appLogo, width, height)
}

func setControlVisible(ctrl *ui.Static, visible bool) {
	if ctrl == nil {
		return
	}
	cmd := co.SW_HIDE
	if visible {
		cmd = co.SW_SHOW
	}
	ctrl.Hwnd().ShowWindow(cmd)
}

// calcContainSize fits a src×src image inside box×box preserving aspect ratio.
func calcContainSize(srcW, srcH, boxW, boxH int32) (int32, int32) {
	if srcW < 1 || srcH < 1 || boxW < 1 || boxH < 1 {
		return boxW, boxH
	}
	// Compare aspect ratios without floats: srcW/srcH vs boxW/boxH.
	if srcW*boxH < boxW*srcH {
		// Height-bound: scale to full box height.
		w := srcW * boxH / srcH
		if w < 1 {
			w = 1
		}
		return w, boxH
	}
	// Width-bound: scale to full box width.
	h := srcH * boxW / srcW
	if h < 1 {
		h = 1
	}
	return boxW, h
}

func calcLogoSize(clientWidth, clientHeight, margin int32) (int32, int32) {
	if appLogo == nil {
		return 0, 0
	}

	logoWidth := appLogo.width
	logoHeight := appLogo.height
	maxLogoHeight := clientHeight * 10 / 100
	if logoHeight > maxLogoHeight {
		logoWidth = logoWidth * maxLogoHeight / logoHeight
		logoHeight = maxLogoHeight
	}
	maxLogoWidth := clientWidth - margin*2
	if logoWidth > maxLogoWidth {
		logoHeight = logoHeight * maxLogoWidth / logoWidth
		logoWidth = maxLogoWidth
	}
	if logoWidth < 1 {
		logoWidth = 1
	}
	if logoHeight < 1 {
		logoHeight = 1
	}
	return logoWidth, logoHeight
}

func resizeWindowContent(wnd *ui.Main, label, logo, socials *ui.Static, showLogo, showSocials bool) {
	clientRect, err := wnd.Hwnd().GetClientRect()
	if err != nil {
		log.Printf("Не удалось получить размер клиентской области: %v", err)
		return
	}

	margin := dipToPx(wnd.Hwnd(), labelMarginDip)
	clientWidth := clientRect.Right - clientRect.Left
	clientHeight := clientRect.Bottom - clientRect.Top

	if showSocials {
		setControlVisible(logo, false)
		setControlVisible(label, false)
		if socials == nil || appSocials == nil {
			setControlVisible(socials, false)
			return
		}
		imgW, imgH := calcContainSize(appSocials.width, appSocials.height, clientWidth, clientHeight)
		imgX := (clientWidth - imgW) / 2
		imgY := (clientHeight - imgH) / 2
		applyImageAtSize(socials, appSocials, imgW, imgH)
		resizeControl(wnd, socials, imgX, imgY, imgW, imgH)
		setControlVisible(socials, true)
		return
	}

	setControlVisible(socials, false)
	setControlVisible(label, true)

	if !showLogo || logo == nil || appLogo == nil {
		setControlVisible(logo, false)
		resizeControl(wnd, label, margin, margin, clientWidth-margin*2, clientHeight-margin*2)
		return
	}

	logoWidth, logoHeight := calcLogoSize(clientWidth, clientHeight, margin)
	logoX := margin + (clientWidth-margin*2-logoWidth)/2
	logoY := margin

	applyLogoAtSize(logo, logoWidth, logoHeight)
	resizeControl(wnd, logo, logoX, logoY, logoWidth, logoHeight)
	setControlVisible(logo, true)

	textY := logoY + logoHeight + margin
	textHeight := clientHeight - textY - margin
	if textHeight < 1 {
		textHeight = 1
	}
	resizeControl(wnd, label, margin, textY, clientWidth-margin*2, textHeight)
}

func resizeControl(wnd *ui.Main, ctrl *ui.Static, x, y, width, height int32) {
	if ctrl == nil {
		return
	}
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	if err := ctrl.Hwnd().SetWindowPos(
		win.HWND(0),
		win.POINT{X: x, Y: y},
		win.SIZE{Cx: width, Cy: height},
		co.SWP_NOZORDER,
	); err != nil {
		log.Printf("Не удалось изменить размер элемента окна: %v", err)
	}
}
