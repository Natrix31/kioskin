// minisign-keygen генерирует пару ключей для подписи релизов.
//
// Публичный ключ печатается в stdout (его вшивают в приложение). Приватные
// части (seed и keyID) записываются в файлы, чтобы не светить их в консоли —
// их нужно положить в секреты GitHub Actions и НИКОГДА не коммитить.
//
//	go run ./tools/minisign-keygen -seed-out seed.txt -keyid-out keyid.txt
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"

	"kioskin/internal/minisig"
)

func main() {
	seedOut := flag.String("seed-out", "minisign_seed.txt", "файл для hex seed приватного ключа")
	keyIDOut := flag.String("keyid-out", "minisign_keyid.txt", "файл для hex идентификатора ключа")
	flag.Parse()

	pub, seed, keyID, err := minisig.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(*seedOut, []byte(hex.EncodeToString(seed)), 0o600); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*keyIDOut, []byte(hex.EncodeToString(keyID)), 0o600); err != nil {
		log.Fatal(err)
	}

	fmt.Println("PUBLIC_KEY:", pub)
	fmt.Printf("seed  -> %s (секрет MINISIGN_SECRET_SEED)\n", *seedOut)
	fmt.Printf("keyID -> %s (секрет MINISIGN_KEY_ID)\n", *keyIDOut)
}
