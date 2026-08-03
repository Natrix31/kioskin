package main

import (
	"strings"
	"testing"
)

func TestFormatMoney(t *testing.T) {
	cases := map[float64]string{
		0:        "0,00",
		89.9:     "89,90",
		179.8:    "179,80",
		1234.5:   "1 234,50",
		1000000:  "1 000 000,00",
		-45.1:    "-45,10",
	}
	for in, want := range cases {
		if got := formatMoney(in); got != want {
			t.Errorf("formatMoney(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatQty(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{2, "2"},
		{1.5, "1,5"},
		{0.75, "0,75"},
		{3.000, "3"},
		{0, "0"},
	}
	for _, c := range cases {
		if got := formatQty(c.in); got != c.want {
			t.Errorf("formatQty(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParsePurchaseListMissingField(t *testing.T) {
	_, err := parsePurchaseList([]byte(`{"goods":[{"name":"Хлеб","price":45.0}]}`))
	if err == nil {
		t.Fatal("ожидалась ошибка о недостающих полях, получено nil")
	}
	if !strings.Contains(err.Error(), "товар #1") {
		t.Errorf("ошибка должна указывать номер товара: %v", err)
	}
	if !strings.Contains(err.Error(), "quantity") || !strings.Contains(err.Error(), "sum") {
		t.Errorf("ошибка должна называть quantity и sum: %v", err)
	}
}

func TestParsePurchaseListNoGoods(t *testing.T) {
	if _, err := parsePurchaseList([]byte(`{}`)); err == nil {
		t.Fatal("ожидалась ошибка об отсутствии goods")
	}
}

func TestParsePurchaseListOK(t *testing.T) {
	body := `{"goods":[
		{"name":" Молоко 3.2% ","quantity":2,"price":89.9,"sum":179.8},
		{"name":"Вода минеральная 1.5 л","quantity":1,"price":52.9,"sum":52.9}
	]}`
	items, err := parsePurchaseList([]byte(body))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("ожидалось 2 позиции, получено %d", len(items))
	}
	if items[0].Name != "Молоко 3.2%" || items[0].Quantity != 2 || items[0].Sum != 179.8 {
		t.Errorf("позиция 0 распарсена неверно: %+v", items[0])
	}
	if items[1].Name != "Вода минеральная 1.5 л" || items[1].Sum != 52.9 {
		t.Errorf("позиция 1 распарсена неверно: %+v", items[1])
	}
}

func TestRenderPurchaseListLineWidth(t *testing.T) {
	items := []purchaseItem{
		{"Молоко 3.2% 950мл", 2, 89.9, 179.8},
		{"Хлеб бородинский нарезной длинное название", 1, 45, 45},
		{"Бананы", 1.25, 99.9, 124.88},
	}
	out := renderPurchaseList(items)
	for i, line := range strings.Split(out, "\n") {
		if n := len([]rune(line)); n != lineWidth {
			t.Errorf("строка %d имеет ширину %d рун, ожидалось %d: %q", i, n, lineWidth, line)
		}
	}
	// Итоговая строка должна содержать сумму всех позиций (349,68).
	if !strings.Contains(out, "349,68") {
		t.Errorf("нет корректного ИТОГО в выводе:\n%s", out)
	}
	t.Logf("\n%s", out)
}
