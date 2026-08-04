package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	updateRepo     = "Natrix31/kioskin"
	updateInterval = time.Hour
	updateAssetExe = "kioskin.exe"
	updateAssetSum = "kioskin.exe.sha256"
	updateUA       = "kioskin-updater"
)

// version — версия сборки. Задаётся при релизной сборке через
// -ldflags "-X main.version=vX.Y.Z". В обычной сборке — "dev", и тогда
// автообновление отключено.
var version = "dev"

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

func (r *ghRelease) assetURL(name string) string {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.URL
		}
	}
	return ""
}

// startUpdater чистит остатки прошлого обновления и, для релизных сборок,
// запускает периодическую проверку новых релизов (при старте и раз в час).
func startUpdater(app *appRuntime) {
	cleanupOldBinary()

	if version == "dev" {
		log.Printf("Сборка dev — автообновление отключено")
		return
	}

	go func() {
		checkForUpdate(app)
		t := time.NewTicker(updateInterval)
		defer t.Stop()
		for range t.C {
			if app.isShuttingDown() {
				return
			}
			checkForUpdate(app)
		}
	}()
}

// cleanupOldBinary удаляет kioskin.exe.old, оставшийся после обновления.
// Старый процесс мог ещё держать файл, поэтому пытаемся несколько раз.
func cleanupOldBinary() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	old := exe + ".old"
	go func() {
		for i := 0; i < 10; i++ {
			if err := os.Remove(old); err == nil || os.IsNotExist(err) {
				return
			}
			time.Sleep(2 * time.Second)
		}
	}()
}

func checkForUpdate(app *appRuntime) {
	rel, err := fetchLatestRelease()
	if err != nil {
		log.Printf("Проверка обновления не удалась: %v", err)
		return
	}
	if rel.TagName == "" || rel.TagName == version {
		return // актуальная версия
	}

	exeURL := rel.assetURL(updateAssetExe)
	sumURL := rel.assetURL(updateAssetSum)
	if exeURL == "" || sumURL == "" {
		log.Printf("В релизе %s нет нужных ассетов (%s, %s)", rel.TagName, updateAssetExe, updateAssetSum)
		return
	}

	data, err := download(exeURL)
	if err != nil {
		log.Printf("Не удалось скачать бинарник: %v", err)
		return
	}
	sumData, err := download(sumURL)
	if err != nil {
		log.Printf("Не удалось скачать контрольную сумму: %v", err)
		return
	}
	wantSum, err := parseChecksum(sumData)
	if err != nil {
		log.Printf("Некорректная контрольная сумма: %v", err)
		return
	}

	gotSum := sha256.Sum256(data)
	if hex.EncodeToString(gotSum[:]) != wantSum {
		log.Printf("SHA-256 не совпал — обновление отклонено")
		return
	}

	log.Printf("Обновление %s -> %s", version, rel.TagName)
	if err := applyUpdate(app, data); err != nil {
		log.Printf("Не удалось применить обновление: %v", err)
	}
}

func fetchLatestRelease() (*ghRelease, error) {
	url := "https://api.github.com/repos/" + updateRepo + "/releases/latest"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", updateUA)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API вернул %d", resp.StatusCode)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func download(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", updateUA)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// parseChecksum извлекает hex-сумму из файла вида "<hex>" или "<hex>  kioskin.exe".
func parseChecksum(data []byte) (string, error) {
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "", fmt.Errorf("пустой файл")
	}
	sum := strings.ToLower(fields[0])
	if len(sum) != 64 {
		return "", fmt.Errorf("ожидалась SHA-256 из 64 символов, получено %d", len(sum))
	}
	return sum, nil
}

// applyUpdate записывает новый бинарник рядом, подменяет текущий exe
// (через переименование, т.к. запущенный exe нельзя перезаписать) и
// перезапускает приложение.
func applyUpdate(app *appRuntime, data []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	newPath := exe + ".new"
	oldPath := exe + ".old"

	if err := os.WriteFile(newPath, data, 0o755); err != nil {
		return fmt.Errorf("запись нового бинарника: %w", err)
	}

	_ = os.Remove(oldPath)
	if err := os.Rename(exe, oldPath); err != nil {
		os.Remove(newPath)
		return fmt.Errorf("переименование текущего exe: %w", err)
	}
	if err := os.Rename(newPath, exe); err != nil {
		os.Rename(oldPath, exe) // откат
		return fmt.Errorf("установка нового exe: %w", err)
	}

	if !app.beginShutdown() {
		return nil // уже завершаемся
	}

	// Освобождаем порт до запуска новой копии.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = app.server.Shutdown(ctx)
	app.state.closeActiveWindow()

	cmd := exec.Command(exe)
	if wd, err := os.Getwd(); err == nil {
		cmd.Dir = wd
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("запуск обновлённого процесса: %w", err)
	}

	log.Printf("Обновление установлено, перезапуск")
	os.Exit(0)
	return nil
}
