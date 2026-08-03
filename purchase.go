package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// purchaseItem — одна позиция чека.
type purchaseItem struct {
	Name     string
	Quantity float64
	Price    float64
	Sum      float64
}

// purchaseInput — одна позиция в теле запроса /update. Указатели нужны, чтобы
// отличить отсутствующее поле от нулевого значения.
type purchaseInput struct {
	Name     *string  `json:"name"`
	Quantity *float64 `json:"quantity"`
	Price    *float64 `json:"price"`
	Sum      *float64 `json:"sum"`
}

// purchaseRequest — тело запроса /update: весь список товаров целиком.
type purchaseRequest struct {
	Goods []purchaseInput `json:"goods"`
}

// parsePurchaseList разбирает JSON тела /update ({"goods": [...]}) и проверяет
// обязательные поля каждой позиции. Возвращает список товаров.
func parsePurchaseList(body []byte) ([]purchaseItem, error) {
	var req purchaseRequest
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(&req); err != nil {
		return nil, fmt.Errorf("некорректный JSON: %v", err)
	}
	if req.Goods == nil {
		return nil, fmt.Errorf("отсутствует обязательное поле goods (список товаров)")
	}

	items := make([]purchaseItem, 0, len(req.Goods))
	for i, in := range req.Goods {
		item, err := validatePurchaseItem(in)
		if err != nil {
			return nil, fmt.Errorf("товар #%d: %v", i+1, err)
		}
		items = append(items, item)
	}
	return items, nil
}

// validatePurchaseItem проверяет обязательные поля одной позиции.
func validatePurchaseItem(in purchaseInput) (purchaseItem, error) {
	var missing []string
	if in.Name == nil || strings.TrimSpace(*in.Name) == "" {
		missing = append(missing, "name")
	}
	if in.Quantity == nil {
		missing = append(missing, "quantity")
	}
	if in.Price == nil {
		missing = append(missing, "price")
	}
	if in.Sum == nil {
		missing = append(missing, "sum")
	}
	if len(missing) > 0 {
		return purchaseItem{}, fmt.Errorf(
			"отсутствуют или пусты обязательные поля: %s", strings.Join(missing, ", "))
	}

	return purchaseItem{
		Name:     strings.TrimSpace(*in.Name),
		Quantity: *in.Quantity,
		Price:    *in.Price,
		Sum:      *in.Sum,
	}, nil
}

// formatMoney форматирует сумму: два знака после запятой, разделитель разрядов
// — пробел, десятичный разделитель — запятая (1 234,50).
func formatMoney(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	cents := int64(v*100 + 0.5)
	rub := cents / 100
	kop := cents % 100

	s := groupThousands(rub) + "," + fmt.Sprintf("%02d", kop)
	if neg {
		s = "-" + s
	}
	return s
}

// formatQty форматирует количество: до 3 знаков после запятой без хвостовых
// нулей, десятичный разделитель — запятая (2, 1,5, 0,75).
func formatQty(v float64) string {
	s := strconv.FormatFloat(v, 'f', 3, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	if s == "" || s == "-0" {
		s = "0"
	}
	return strings.Replace(s, ".", ",", 1)
}

// groupThousands вставляет пробелы между разрядами тысяч (1234567 -> 1 234 567).
func groupThousands(n int64) string {
	digits := strconv.FormatInt(n, 10)
	if len(digits) <= 3 {
		return digits
	}
	var b strings.Builder
	pre := len(digits) % 3
	if pre > 0 {
		b.WriteString(digits[:pre])
	}
	for i := pre; i < len(digits); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}
