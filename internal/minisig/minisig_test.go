package minisig

import (
	"strings"
	"testing"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	pub, seed, keyID, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if !strings.HasPrefix(pub, "RW") {
		t.Errorf("публичный ключ minisign должен начинаться с RW, получено %q", pub[:2])
	}

	msg := []byte("двоичное содержимое \x00\x01\x02 kioskin.exe")
	sig, err := Sign(seed, keyID, msg, "kioskin v1.2.3", "timestamp:1700000000 file:kioskin.exe")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if err := Verify(pub, msg, []byte(sig)); err != nil {
		t.Fatalf("Verify корректной подписи: %v", err)
	}
}

func TestVerifyRejectsTamperedMessage(t *testing.T) {
	pub, seed, keyID, _ := GenerateKey()
	msg := []byte("оригинал")
	sig, _ := Sign(seed, keyID, msg, "c", "tc")
	if err := Verify(pub, []byte("подделка"), []byte(sig)); err == nil {
		t.Fatal("ожидалась ошибка на изменённом сообщении")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	_, seed, keyID, _ := GenerateKey()
	otherPub, _, _, _ := GenerateKey() // другой ключ
	msg := []byte("данные")
	sig, _ := Sign(seed, keyID, msg, "c", "tc")
	if err := Verify(otherPub, msg, []byte(sig)); err == nil {
		t.Fatal("ожидалась ошибка на чужом публичном ключе")
	}
}

func TestVerifyRejectsTamperedTrustedComment(t *testing.T) {
	pub, seed, keyID, _ := GenerateKey()
	msg := []byte("данные")
	sig, _ := Sign(seed, keyID, msg, "c", "оригинальный комментарий")
	// подменяем строку доверенного комментария, оставляя всё остальное
	tampered := strings.Replace(sig, "оригинальный комментарий", "подменённый комментарий", 1)
	if err := Verify(pub, msg, []byte(tampered)); err == nil {
		t.Fatal("ожидалась ошибка на подменённом доверенном комментарии")
	}
}
