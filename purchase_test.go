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

func TestParsePurchaseItemMissingFields(t *testing.T) {
	_, err := parsePurchaseItem([]byte(`{"name":"Хлеб","price":45.0}`))
	if err == nil {
		t.Fatal("ожидалась ошибка о недостающих полях, получено nil")
	}
	if !strings.Contains(err.Error(), "quantity") || !strings.Contains(err.Error(), "sum") {
		t.Errorf("ошибка должна называть quantity и sum: %v", err)
	}
}

func TestParsePurchaseItemOK(t *testing.T) {
	it, err := parsePurchaseItem([]byte(`{"name":" Молоко 3.2% ","quantity":2,"price":89.9,"sum":179.8}`))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if it.Name != "Молоко 3.2%" || it.Quantity != 2 || it.Price != 89.9 || it.Sum != 179.8 {
		t.Errorf("распарсено неверно: %+v", it)
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
