package main

import (
	"strings"
	"syscall"
	"unsafe"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/win"
)

// Отступы внутри ячеек и межстрочный интервал (px при 96 DPI).
const (
	cellPadX = 8
	cellPadY = 5
	lineGap  = 4
)

var (
	procCreateSolidBrush = syscall.NewLazyDLL("gdi32.dll").NewProc("CreateSolidBrush")
	// FillRect живёт в user32.dll (обёртка windigo ошибочно ищет её в gdi32).
	procFillRect = syscall.NewLazyDLL("user32.dll").NewProc("FillRect")
)

func solidBrush(c win.COLORREF) win.HBRUSH {
	r, _, _ := procCreateSolidBrush.Call(uintptr(c))
	return win.HBRUSH(r)
}

// Цвета текста.
var (
	clrText   = win.RGB(30, 30, 30)    // обычные строки
	clrHeadTx = win.RGB(255, 255, 255) // шапка
	clrTotTx  = win.RGB(255, 255, 255) // ИТОГО
)

// Кисти фона (создаются один раз, живут до завершения процесса).
var (
	brBg     win.HBRUSH // общий фон
	brRowA   win.HBRUSH // чётная строка
	brRowB   win.HBRUSH // нечётная строка (зебра)
	brHeader win.HBRUSH // шапка
	brTotal  win.HBRUSH // строка ИТОГО
)

func ensureBrushes() {
	if brBg != 0 {
		return
	}
	// Зелёная палитра в тон логотипа «Table.Точка» (зелёный ≈ RGB 0,155,60).
	brBg = solidBrush(win.RGB(255, 255, 255))
	brRowA = solidBrush(win.RGB(245, 251, 247)) // почти белый с зелёным оттенком
	brRowB = solidBrush(win.RGB(223, 242, 230)) // светло-зелёный (зебра)
	brHeader = solidBrush(win.RGB(0, 155, 60))  // зелёный логотипа
	brTotal = solidBrush(win.RGB(0, 120, 46))   // насыщенный зелёный для ИТОГО
}

// drawPurchaseList рисует таблицу списка покупок в прямоугольнике rc на hdc:
// шапка, строки с чередующимся фоном (зебра) и переносом длинных наименований,
// строка ИТОГО.
func drawPurchaseList(hdc win.HDC, rc win.RECT, items []purchaseItem) {
	ensureBrushes()

	// Фон всей области контрола.
	fillRow(hdc, rc.Left, rc.Top, rc.Right, rc.Bottom, brBg)

	if len(items) == 0 {
		return
	}

	if f := ensureListFont(); f != 0 {
		hdc.SelectObjectFont(f)
	}
	hdc.SetBkMode(co.BKMODE_TRANSPARENT)

	lineH, avg := int32(24), int32(12)
	if tm, err := hdc.GetTextMetrics(); err == nil {
		lineH = int32(tm.Height) + lineGap
		avg = int32(tm.AveCharWidth)
	}

	// Геометрия колонок: числовые фиксированы по средней ширине символа,
	// наименование занимает остаток.
	qtyW := avg * 8
	priceW := avg * 12
	sumW := avg * 13
	nameW := (rc.Right - rc.Left) - qtyW - priceW - sumW
	if nameW < avg*6 {
		nameW = avg * 6
	}
	nameX := rc.Left
	qtyRight := nameX + nameW + qtyW
	priceRight := qtyRight + priceW
	sumRight := priceRight + sumW

	var total float64
	for _, it := range items {
		total += it.Sum
	}

	y := rc.Top

	// Шапка.
	headerH := lineH + 2*cellPadY
	fillRow(hdc, rc.Left, y, rc.Right, y+headerH, brHeader)
	hdc.SetTextColor(clrHeadTx)
	hdc.TextOut(int(nameX+cellPadX), int(y+cellPadY), "НАИМЕНОВАНИЕ")
	drawRight(hdc, "КОЛ-ВО", qtyRight, y+cellPadY)
	drawRight(hdc, "ЦЕНА", priceRight, y+cellPadY)
	drawRight(hdc, "СУММА", sumRight, y+cellPadY)
	y += headerH

	// Позиции.
	for i, it := range items {
		lines := wrapText(hdc, it.Name, nameW-2*cellPadX)
		rowH := int32(len(lines))*lineH + 2*cellPadY
		if y+rowH > rc.Bottom {
			break // не влезает — прекращаем (окно ограничено по высоте)
		}

		br := brRowA
		if i%2 == 1 {
			br = brRowB
		}
		fillRow(hdc, rc.Left, y, rc.Right, y+rowH, br)

		hdc.SetTextColor(clrText)
		ty := y + cellPadY
		for _, ln := range lines {
			hdc.TextOut(int(nameX+cellPadX), int(ty), ln)
			ty += lineH
		}
		drawRight(hdc, formatQty(it.Quantity), qtyRight, y+cellPadY)
		drawRight(hdc, formatMoney(it.Price), priceRight, y+cellPadY)
		drawRight(hdc, formatMoney(it.Sum), sumRight, y+cellPadY)

		y += rowH
	}

	// Строка ИТОГО.
	if totalH := lineH + 2*cellPadY; y+totalH <= rc.Bottom {
		fillRow(hdc, rc.Left, y, rc.Right, y+totalH, brTotal)
		hdc.SetTextColor(clrTotTx)
		hdc.TextOut(int(nameX+cellPadX), int(y+cellPadY), "ИТОГО")
		drawRight(hdc, formatMoney(total), sumRight, y+cellPadY)
	}
}

func fillRow(hdc win.HDC, left, top, right, bottom int32, br win.HBRUSH) {
	rc := win.RECT{Left: left, Top: top, Right: right, Bottom: bottom}
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&rc)), uintptr(br))
}

// drawRight выводит строку, выровненную по правому краю колонки colRight.
func drawRight(hdc win.HDC, s string, colRight, top int32) {
	hdc.TextOut(int(colRight-cellPadX-textWidth(hdc, s)), int(top), s)
}

func textWidth(hdc win.HDC, s string) int32 {
	if s == "" {
		return 0
	}
	if sz, err := hdc.GetTextExtentPoint32(s); err == nil {
		return sz.Cx
	}
	return 0
}

// wrapText разбивает строку на строки, помещающиеся в maxW пикселей по ширине.
// Слова длиннее maxW переносятся посимвольно.
func wrapText(hdc win.HDC, text string, maxW int32) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	cur := ""
	flush := func() {
		if cur != "" {
			lines = append(lines, cur)
			cur = ""
		}
	}

	for _, word := range words {
		candidate := word
		if cur != "" {
			candidate = cur + " " + word
		}
		if textWidth(hdc, candidate) <= maxW {
			cur = candidate
			continue
		}
		flush()
		if textWidth(hdc, word) <= maxW {
			cur = word
			continue
		}
		// Слово шире колонки — режем по символам.
		chunk := ""
		for _, r := range word {
			test := chunk + string(r)
			if textWidth(hdc, test) <= maxW {
				chunk = test
				continue
			}
			if chunk != "" {
				lines = append(lines, chunk)
			}
			chunk = string(r)
		}
		cur = chunk
	}
	flush()

	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}
