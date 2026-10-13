package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed frontend/dist
var assets embed.FS

type App struct {
	vault *Vault
	mux   *http.ServeMux
	menu  *menu.Menu
}

func NewApp() *App {
	v := NewVault()
	m := http.NewServeMux()
	m.HandleFunc("/api/entries", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			entries, err := v.GetEntries()
			if err != nil {
				http.Error(w, err.Error(), http.StatusLocked)
				return
			}
			json.NewEncoder(w).Encode(entries)
		case http.MethodPost:
			var e Entry
			if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			entry, err := v.AddEntry(e.Name, e.Value, e.Kind)
			if err != nil {
				http.Error(w, err.Error(), http.StatusLocked)
				return
			}
			json.NewEncoder(w).Encode(entry)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	m.HandleFunc("/api/entries/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/entries/")
		switch r.Method {
		case http.MethodPut:
			var e Entry
			if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			entry, err := v.UpdateEntry(id, e.Name, e.Value, e.Kind)
			if err != nil {
				http.Error(w, err.Error(), http.StatusLocked)
				return
			}
			json.NewEncoder(w).Encode(entry)
		case http.MethodDelete:
			if err := v.DeleteEntry(id); err != nil {
				http.Error(w, err.Error(), http.StatusLocked)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	return &App{vault: v, mux: m}
}

func (a *App) startup(ctx context.Context) {
	a.vault.startup(ctx)
	runtime.ResetSignalHandlers()

	accUnlock, _ := keys.Parse("CmdOrCtrl+Shift+U")
	accLock, _ := keys.Parse("CmdOrCtrl+L")
	accNew, _ := keys.Parse("CmdOrCtrl+N")
	accTheme, _ := keys.Parse("CmdOrCtrl+D")
	accDevtools, _ := keys.Parse("CmdOrCtrl+Shift+I")
	accAbout, _ := keys.Parse("CmdOrCtrl+Shift+A")

	unlockItem := menu.Text("Unlock", accUnlock, func(_ *menu.CallbackData) {
		runtime.EventsEmit(ctx, "menu:unlock")
	})
	lockItem := menu.Text("Lock", accLock, func(_ *menu.CallbackData) {
		a.doLock(ctx)
	})
	newItem := menu.Text("New Entry", accNew, func(_ *menu.CallbackData) {
		runtime.EventsEmit(ctx, "menu:new-entry")
	})
	themeItem := menu.Checkbox("Dark Mode", true, accTheme, func(_ *menu.CallbackData) {
		runtime.EventsEmit(ctx, "menu:toggle-theme")
	})
	devtoolsItem := menu.Text("Developer Tools", accDevtools, func(_ *menu.CallbackData) {
		runtime.EventsEmit(ctx, "menu:devtools")
	})
	aboutItem := menu.Text("About", accAbout, func(_ *menu.CallbackData) {
		runtime.EventsEmit(ctx, "menu:about")
	})

	vaultSub := menu.NewMenu()
	vaultSub.Append(unlockItem)
	vaultSub.Append(menu.Separator())
	vaultSub.Append(lockItem)
	vaultSub.Append(menu.Separator())
	vaultSub.Append(newItem)
	vaultSub.Append(menu.Separator())
	vaultSub.Append(themeItem)
	vaultSub.Append(menu.Separator())
	vaultSub.Append(devtoolsItem)
	vaultSub.Append(aboutItem)

	a.menu = menu.NewMenuFromItems(
		menu.AppMenu(),
		menu.EditMenu(),
		menu.SubMenu("Vault", vaultSub),
	)
	runtime.MenuSetApplicationMenu(ctx, a.menu)

	runtime.EventsOn(ctx, "menu:unlock", func(_ ...interface{}) {
		runtime.WindowSetTitle(ctx, "Vault — unlock")
	})
	runtime.EventsOn(ctx, "menu:new-entry", func(_ ...interface{}) {
		runtime.WindowSetTitle(ctx, "Vault — new entry")
	})
	runtime.EventsOn(ctx, "menu:toggle-theme", func(_ ...interface{}) {
		runtime.EventsEmit(ctx, "menu:toggle-theme")
	})
	runtime.EventsOn(ctx, "menu:devtools", func(_ ...interface{}) {
		runtime.EventsEmit(ctx, "menu:devtools")
	})
	runtime.EventsOn(ctx, "menu:about", func(_ ...interface{}) {
		runtime.EventsEmit(ctx, "menu:about")
	})
}

func (a *App) doLock(ctx context.Context) {
	if err := a.vault.Lock(); err != nil {
		runtime.LogError(ctx, fmt.Sprintf("failed to lock: %v", err))
		return
	}
	runtime.EventsEmit(ctx, "vault:lock")
	runtime.WindowSetTitle(ctx, "Vault — locked")
}

func apiInterceptor(m *http.ServeMux) assetserver.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				rec := httptest.NewRecorder()
				m.ServeHTTP(rec, r)
				w.Header().Set("Content-Type", rec.Header().Get("Content-Type"))
				w.WriteHeader(rec.Code)
				if rec.Body.Len() > 0 {
					w.Write(rec.Body.Bytes())
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:  "Vault",
		Width:  900,
		Height: 640,
		AssetServer: &assetserver.Options{
			Assets:   fs.FS(assets),
			Middleware: assetserver.ChainMiddleware(
				apiInterceptor(app.mux),
			),
		},
		BackgroundColour: &options.RGBA{R: 20, G: 20, B: 30, A: 255},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app.vault,
		},
	})
	if err != nil {
		fmt.Println("Error:", err)
	}
}

type responseReader struct {
	body        io.ReadCloser
	statusCode  int
	contentType string
}

func (r *responseReader) Read(p []byte) (n int, err error) {
	return r.body.Read(p)
}

func (r *responseReader) Close() error {
	return r.body.Close()
}

func (r *responseReader) StatusCode() int {
	return r.statusCode
}

func (r *responseReader) ContentType() string {
	return r.contentType
}
