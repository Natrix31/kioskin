// minisign-sign подписывает файл в формате minisign. Используется в CI.
//
// Приватный ключ читается из переменных окружения (секретов):
//
//	MINISIGN_SECRET_SEED — hex 32-байтного seed
//	MINISIGN_KEY_ID      — hex 8-байтного идентификатора ключа
//
//	go run ./tools/minisign-sign -in kioskin.exe -out kioskin.exe.minisig -comment "kioskin v1.2.3"
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kioskin/internal/minisig"
)

func main() {
	in := flag.String("in", "", "подписываемый файл")
	out := flag.String("out", "", "файл подписи (.minisig)")
	comment := flag.String("comment", "kioskin", "недоверенный комментарий")
	flag.Parse()

	if *in == "" || *out == "" {
		log.Fatal("нужны -in и -out")
	}

	seed, err := hex.DecodeString(strings.TrimSpace(os.Getenv("MINISIGN_SECRET_SEED")))
	if err != nil || len(seed) == 0 {
		log.Fatal("MINISIGN_SECRET_SEED: некорректный или пустой hex")
	}
	keyID, err := hex.DecodeString(strings.TrimSpace(os.Getenv("MINISIGN_KEY_ID")))
	if err != nil || len(keyID) == 0 {
		log.Fatal("MINISIGN_KEY_ID: некорректный или пустой hex")
	}

	msg, err := os.ReadFile(*in)
	if err != nil {
		log.Fatal(err)
	}

	trusted := fmt.Sprintf("timestamp:%d file:%s", time.Now().Unix(), filepath.Base(*in))
	sig, err := minisig.Sign(seed, keyID, msg, *comment, trusted)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, []byte(sig), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("подписан %s -> %s\n", *in, *out)
}
