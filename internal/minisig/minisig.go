// Package minisig реализует создание и проверку подписей в формате minisign
// (https://jedisct1.github.io/minisign/) на чистом Ed25519 (без предварительного
// хеширования BLAKE2b — алгоритм "Ed"). Подписи совместимы с CLI minisign.
package minisig

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// algEd — идентификатор алгоритма «чистый Ed25519» в minisign.
var algEd = [2]byte{'E', 'd'}

const trustedCommentPrefix = "trusted comment: "

// GenerateKey создаёт новую пару ключей.
// Возвращает публичный ключ в base64-формате minisign (строка, начинается с "RW"),
// 32-байтный seed приватного ключа и 8-байтный идентификатор ключа.
func GenerateKey() (pubBase64 string, seed, keyID []byte, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", nil, nil, err
	}
	keyID = make([]byte, 8)
	if _, err := rand.Read(keyID); err != nil {
		return "", nil, nil, err
	}
	return encodePublicKey(keyID, pub), priv.Seed(), keyID, nil
}

func encodePublicKey(keyID []byte, pub ed25519.PublicKey) string {
	buf := make([]byte, 0, 2+8+ed25519.PublicKeySize)
	buf = append(buf, algEd[:]...)
	buf = append(buf, keyID...)
	buf = append(buf, pub...)
	return base64.StdEncoding.EncodeToString(buf)
}

func decodePublicKey(pubBase64 string) (keyID [8]byte, pub ed25519.PublicKey, err error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(pubBase64))
	if err != nil {
		return keyID, nil, fmt.Errorf("публичный ключ: base64: %w", err)
	}
	if len(raw) != 2+8+ed25519.PublicKeySize {
		return keyID, nil, fmt.Errorf("публичный ключ: неверная длина %d", len(raw))
	}
	if raw[0] != algEd[0] || raw[1] != algEd[1] {
		return keyID, nil, errors.New("публичный ключ: неподдерживаемый алгоритм")
	}
	copy(keyID[:], raw[2:10])
	pub = ed25519.PublicKey(raw[10:])
	return keyID, pub, nil
}

// Sign подписывает message ключом (seed + keyID) и возвращает содержимое
// файла подписи .minisig.
func Sign(seed, keyID, message []byte, untrustedComment, trustedComment string) (string, error) {
	if len(seed) != ed25519.SeedSize {
		return "", fmt.Errorf("seed: ожидалось %d байт, получено %d", ed25519.SeedSize, len(seed))
	}
	if len(keyID) != 8 {
		return "", fmt.Errorf("keyID: ожидалось 8 байт, получено %d", len(keyID))
	}
	if strings.ContainsAny(untrustedComment, "\r\n") || strings.ContainsAny(trustedComment, "\r\n") {
		return "", errors.New("комментарий не должен содержать перевод строки")
	}

	priv := ed25519.NewKeyFromSeed(seed)
	sig := ed25519.Sign(priv, message)

	blob := make([]byte, 0, 2+8+len(sig))
	blob = append(blob, algEd[:]...)
	blob = append(blob, keyID...)
	blob = append(blob, sig...)

	// Глобальная подпись покрывает подпись файла + доверенный комментарий.
	global := ed25519.Sign(priv, append(append([]byte{}, sig...), []byte(trustedComment)...))

	var b strings.Builder
	b.WriteString("untrusted comment: " + untrustedComment + "\n")
	b.WriteString(base64.StdEncoding.EncodeToString(blob) + "\n")
	b.WriteString(trustedCommentPrefix + trustedComment + "\n")
	b.WriteString(base64.StdEncoding.EncodeToString(global) + "\n")
	return b.String(), nil
}

// Verify проверяет, что sigFile — корректная minisign-подпись message,
// сделанная ключом pubBase64. Возвращает nil при успехе.
func Verify(pubBase64 string, message, sigFile []byte) error {
	keyID, pub, err := decodePublicKey(pubBase64)
	if err != nil {
		return err
	}

	lines := strings.Split(strings.ReplaceAll(string(sigFile), "\r\n", "\n"), "\n")
	if len(lines) < 4 {
		return errors.New("подпись: слишком мало строк")
	}

	blob, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[1]))
	if err != nil {
		return fmt.Errorf("подпись: base64: %w", err)
	}
	if len(blob) != 2+8+ed25519.SignatureSize {
		return fmt.Errorf("подпись: неверная длина %d", len(blob))
	}
	if blob[0] != algEd[0] || blob[1] != algEd[1] {
		return errors.New("подпись: неподдерживаемый алгоритм (нужен чистый Ed)")
	}
	var sigKeyID [8]byte
	copy(sigKeyID[:], blob[2:10])
	if sigKeyID != keyID {
		return errors.New("подпись: идентификатор ключа не совпадает")
	}
	sig := blob[10:]
	if !ed25519.Verify(pub, message, sig) {
		return errors.New("подпись: недействительна")
	}

	trusted, ok := strings.CutPrefix(lines[2], trustedCommentPrefix)
	if !ok {
		return errors.New("подпись: отсутствует доверенный комментарий")
	}
	global, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[3]))
	if err != nil {
		return fmt.Errorf("подпись: base64 глобальной подписи: %w", err)
	}
	if len(global) != ed25519.SignatureSize {
		return fmt.Errorf("подпись: неверная длина глобальной подписи %d", len(global))
	}
	if !ed25519.Verify(pub, append(append([]byte{}, sig...), []byte(trusted)...), global) {
		return errors.New("подпись: недействительна подпись доверенного комментария")
	}
	return nil
}
