package main

import "kioskin/internal/minisig"

// minisignPublicKey — публичный ключ (формат minisign), которым подписываются
// релизы. Приватный ключ хранится в секретах GitHub Actions (MINISIGN_SECRET_SEED,
// MINISIGN_KEY_ID) и в репозитории отсутствует. Чтобы сменить ключ — сгенерируйте
// новый через ./tools/minisign-keygen и обновите эту строку и секреты.
const minisignPublicKey = "RWQtLJ/cgoBm7p8jgTJeHvyi2OiOY2rGKDNaXUa5Y2Ct/IhfeGfuiDdS"

// verifyReleaseSignature проверяет minisign-подпись скачанного бинарника.
func verifyReleaseSignature(binary, signature []byte) error {
	return minisig.Verify(minisignPublicKey, binary, signature)
}
