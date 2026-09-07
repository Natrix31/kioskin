package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

// logFileName — имя файла лога рядом с исполняемым файлом.
const logFileName = "kioskin.log"

// initLogging направляет стандартный лог в текстовый файл рядом с бинарником
// (kioskin.log, в режиме дозаписи). Если открыть файл не удалось — лог
// отбрасывается, чтобы GUI-приложение не падало и не открывало консоль.
func initLogging() {
	log.SetFlags(log.LstdFlags) // дата и время в каждой строке

	f, err := os.OpenFile(logFilePath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.SetOutput(io.Discard)
		return
	}
	log.SetOutput(f)
	log.Printf("=== Kioskin %s запущен ===", version)
}

// logFilePath возвращает путь к файлу лога рядом с бинарником; если каталог
// определить не удалось — в рабочей директории.
func logFilePath() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), logFileName)
	}
	return logFileName
}
