package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	initialWindowText = "."
	lwaAlpha          = 0x2
	alphaTransparent  = 50
	alphaOpaque       = 255
	labelMarginDip    = 12
	configFilePath    = "config.json"
)

var (
	user32ProcSetLayeredWindowAttributes = syscall.NewLazyDLL("user32.dll").NewProc("SetLayeredWindowAttributes")
)

type windowState struct {
	mu       sync.RWMutex
	text     string
	alpha    byte
	showLogo bool
	wnd      *ui.Main
	label    *ui.Static
	logo     *ui.Static
}

type appConfig struct {
	MonitorIndex int    `json:"monitor_index"`
	LogoPath     string `json:"logo_path"`
}

type windowPlacement struct {
	x      int32
	y      int32
	width  int32
	height int32
}

type monitorInfo struct {
	Index  int   `json:"index"`
	Left   int32 `json:"left"`
	Top    int32 `json:"top"`
	Right  int32 `json:"right"`
	Bottom int32 `json:"bottom"`
	Width  int32 `json:"width"`
	Height int32 `json:"height"`
}

type appRuntime struct {
	server       *http.Server
	state        *windowState
	shuttingDown atomic.Bool
}

func newWindowState() *windowState {
	return &windowState{
		text:  initialWindowText,
		alpha: alphaTransparent,
	}
}

func main() {
	runtime.LockOSThread() // Windows GUI must run on one OS thread.

	cfg := loadConfig(configFilePath)
	if err := initAppLogo(cfg); err != nil {
		log.Printf("Не удалось загрузить логотип: %v", err)
	}
	state := newWindowState()
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    ":8282",
		Handler: mux,
	}
	runtimeState := &appRuntime{
		server: server,
		state:  state,
	}
	mux.HandleFunc("/update", handleShowRequest(state))
	mux.HandleFunc("/clear", handleClearRequest(state))
	mux.HandleFunc("/health", handleHealthcheck())
	mux.HandleFunc("/monitors", handleMonitors())
	mux.HandleFunc("/shutdown", handleShutdown(runtimeState))
	registerSwaggerRoutes(mux)

	go func() {
		//log.Println("HTTP сервер запущен на http://localhost:8282")
		//log.Println("Swagger UI: http://localhost:8282/docs")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Ошибка HTTP сервера: %v", err)
		}
	}()

	for !runtimeState.isShuttingDown() {
		ShowMainWindow(state, cfg)
	}
}

// Displays the main window, blocking until it is closed.
func ShowMainWindow(state *windowState, cfg appConfig) int {
	initialText, initialAlpha, initialShowLogo := state.snapshot()
	placement := initialWindowPlacement(cfg)

	wnd := ui.NewMain(
		ui.OptsMain().
			Title("Kioskin").
			Size(int(placement.width), int(placement.height)).
			Style(co.WS_POPUP | co.WS_VISIBLE),
	)

	logoStatic := ui.NewStatic(
		wnd,
		ui.OptsStatic().
			CtrlStyle(co.SS_BITMAP).
			WndStyle(co.WS_CHILD).
			Size(ui.Dpi(200, 80)).
			Position(ui.Dpi(10, 12)),
	)

	lbl := ui.NewStatic(
		wnd,
		ui.OptsStatic().
			Text(initialText).
			Size(ui.Dpi(580, 760)).
			Position(ui.Dpi(10, 22)),
	)

	wnd.On().WmCreate(func(_ ui.WmCreate) int {
		state.bindWindow(wnd, lbl, logoStatic)
		applyWindowPlacement(wnd, placement)
		resizeWindowContent(wnd, lbl, logoStatic, initialShowLogo)
		if err := applyWindowOpacity(wnd, initialAlpha); err != nil {
			log.Printf("Не удалось применить прозрачность окна: %v", err)
		}
		return 0
	})
	wnd.On().WmSize(func(_ ui.WmSize) {
		showLogo := state.isLogoVisible()
		resizeWindowContent(wnd, lbl, logoStatic, showLogo)
	})
	wnd.On().WmDestroy(func() {
		state.unbindWindow(wnd)
	})

	return wnd.RunAsMain()
}

func handleShowRequest(state *windowState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Ошибка чтения тела запроса", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, body, "", "  "); err != nil {
			prettyJSON.WriteString(string(body))
		}

		state.update(prettyJSON.String(), alphaOpaque, true)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "JSON получен: окно обновлено и сделано непрозрачным")
	}
}

func handleClearRequest(state *windowState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		state.update(initialWindowText, alphaTransparent, false)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Окно очищено и переведено в полупрозрачный режим")
	}
}

func handleHealthcheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	}
}

func handleMonitors() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		monitors, err := listMonitors()
		if err != nil {
			http.Error(w, "Не удалось получить список мониторов", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(monitors); err != nil {
			log.Printf("Не удалось записать ответ /monitors: %v", err)
		}
	}
}

func handleShutdown(app *appRuntime) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}
		if !app.beginShutdown() {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "Graceful shutdown уже выполняется")
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Graceful shutdown запущен")

		go app.shutdown()
	}
}

func (s *windowState) snapshot() (string, byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.text, s.alpha, s.showLogo
}

func (s *windowState) isLogoVisible() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.showLogo
}

func (s *windowState) bindWindow(wnd *ui.Main, label, logo *ui.Static) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wnd = wnd
	s.label = label
	s.logo = logo
}

func (s *windowState) unbindWindow(wnd *ui.Main) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.wnd == wnd {
		s.wnd = nil
		s.label = nil
		s.logo = nil
	}
}

func (s *windowState) update(text string, alpha byte, showLogo bool) {
	s.mu.Lock()
	s.text = text
	s.alpha = alpha
	s.showLogo = showLogo
	wnd := s.wnd
	label := s.label
	logo := s.logo
	s.mu.Unlock()

	if wnd == nil || label == nil {
		return
	}

	wnd.UiThread(func() {
		if err := label.Hwnd().SetWindowText(text); err != nil {
			log.Printf("Не удалось обновить текст окна: %v", err)
		}
		resizeWindowContent(wnd, label, logo, showLogo)
		if err := applyWindowOpacity(wnd, alpha); err != nil {
			log.Printf("Не удалось сделать окно непрозрачным: %v", err)
		}
	})
}

func (s *windowState) closeActiveWindow() {
	s.mu.RLock()
	wnd := s.wnd
	s.mu.RUnlock()
	if wnd == nil {
		return
	}

	wnd.UiThread(func() {
		if err := wnd.Hwnd().DestroyWindow(); err != nil {
			log.Printf("Не удалось закрыть окно во время shutdown: %v", err)
		}
	})
}

func dipToPx(hwnd win.HWND, dip int32) int32 {
	dpi := int32(hwnd.GetDpiForWindow())
	if dpi <= 0 {
		dpi = 96
	}
	return (dip * dpi) / 96
}

func loadConfig(path string) appConfig {
	cfg := appConfig{MonitorIndex: 0}

	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Конфиг %s не прочитан (%v), используется монитор 0", path, err)
		return cfg
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("Конфиг %s содержит ошибку (%v), используется монитор 0", path, err)
		return appConfig{MonitorIndex: 0}
	}
	if cfg.MonitorIndex < 0 {
		log.Printf("monitor_index не может быть отрицательным, используется монитор 0")
		cfg.MonitorIndex = 0
	}

	return cfg
}

func initialWindowPlacement(cfg appConfig) windowPlacement {
	if rect, ok := monitorRectByIndex(cfg.MonitorIndex); ok {
		return windowPlacement{
			x:      rect.Left,
			y:      rect.Bottom / 2,
			width:  rect.Right - rect.Left,
			height: (rect.Bottom - rect.Top) / 2,
		}
	}

	screenWidth := win.GetSystemMetrics(co.SM_CXSCREEN)
	screenHeight := win.GetSystemMetrics(co.SM_CYSCREEN)
	if screenWidth <= 0 {
		screenWidth = 600
	}
	if screenHeight <= 0 {
		screenHeight = 600
	}
	log.Printf("Монитор с индексом %d не найден, используется основной экран", cfg.MonitorIndex)

	return windowPlacement{
		x:      0,
		y:      0,
		width:  screenWidth,
		height: screenHeight / 2,
	}
}

func monitorRectByIndex(index int) (win.RECT, bool) {
	monitors, err := listMonitorRects()
	if err != nil {
		log.Printf("Не удалось получить список мониторов: %v", err)
		return win.RECT{}, false
	}
	if index < 0 || index >= len(monitors) {
		return win.RECT{}, false
	}
	return monitors[index], true
}

func applyWindowPlacement(wnd *ui.Main, placement windowPlacement) {
	if err := wnd.Hwnd().SetWindowPos(
		win.HWND(0),
		win.POINT{X: placement.x, Y: placement.y},
		win.SIZE{Cx: placement.width, Cy: placement.height},
		co.SWP_NOZORDER,
	); err != nil {
		log.Printf("Не удалось задать позицию окна: %v", err)
	}
}

func listMonitorRects() ([]win.RECT, error) {
	monitors, err := win.HDC(0).EnumDisplayMonitors(nil)
	if err != nil {
		return nil, err
	}

	rects := make([]win.RECT, 0, len(monitors))
	for _, mon := range monitors {
		rects = append(rects, mon.Rc)
	}
	return rects, nil
}

func listMonitors() ([]monitorInfo, error) {
	rects, err := listMonitorRects()
	if err != nil {
		return nil, err
	}

	result := make([]monitorInfo, 0, len(rects))
	for i, rect := range rects {
		result = append(result, monitorInfo{
			Index:  i,
			Left:   rect.Left,
			Top:    rect.Top,
			Right:  rect.Right,
			Bottom: rect.Bottom,
			Width:  rect.Right - rect.Left,
			Height: rect.Bottom - rect.Top,
		})
	}
	return result, nil
}

func (a *appRuntime) beginShutdown() bool {
	return a.shuttingDown.CompareAndSwap(false, true)
}

func (a *appRuntime) isShuttingDown() bool {
	return a.shuttingDown.Load()
}

func (a *appRuntime) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		log.Printf("Ошибка graceful shutdown HTTP сервера: %v", err)
	}
	a.state.closeActiveWindow()
}

func applyWindowOpacity(wnd *ui.Main, alpha byte) error {
	hwnd := wnd.Hwnd()
	exStyle, err := hwnd.GetWindowLongPtr(co.GWLP_EXSTYLE)
	if err != nil {
		return err
	}

	layered := exStyle | uintptr(co.WS_EX_LAYERED)
	if _, err = hwnd.SetWindowLongPtr(co.GWLP_EXSTYLE, layered); err != nil {
		return err
	}

	ret, _, callErr := user32ProcSetLayeredWindowAttributes.Call(
		uintptr(hwnd),
		0,
		uintptr(alpha),
		lwaAlpha,
	)
	if ret == 0 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return syscall.EINVAL
	}
	return nil
}
