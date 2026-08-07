// minisign-verify проверяет minisign-подпись файла заданным публичным ключом.
// Полезно для ручной проверки скачанного релиза.
//
//	go run ./tools/minisign-verify -pubkey RW... -in kioskin.exe -sig kioskin.exe.minisig
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"kioskin/internal/minisig"
)

func main() {
	pubkey := flag.String("pubkey", "", "публичный ключ (формат minisign, RW...)")
	in := flag.String("in", "", "проверяемый файл")
	sig := flag.String("sig", "", "файл подписи (.minisig)")
	flag.Parse()

	if *pubkey == "" || *in == "" || *sig == "" {
		log.Fatal("нужны -pubkey, -in и -sig")
	}

	data, err := os.ReadFile(*in)
	if err != nil {
		log.Fatal(err)
	}
	sigData, err := os.ReadFile(*sig)
	if err != nil {
		log.Fatal(err)
	}

	if err := minisig.Verify(*pubkey, data, sigData); err != nil {
		fmt.Println("НЕДЕЙСТВИТЕЛЬНА:", err)
		os.Exit(1)
	}
	fmt.Println("OK: подпись действительна")
}
