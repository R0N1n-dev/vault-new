// Coffer is a local-first password vault.
//
// Everything lives in one encrypted file on this machine: there is no server,
// no account and no telemetry. See README.md for the security model.
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     appName,
		Width:     1180,
		Height:    800,
		MinWidth:  960,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// Matches the dark theme's window background so the first paint does not
		// flash white.
		BackgroundColour: &options.RGBA{R: 12, G: 14, B: 22, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		// Two instances writing the same vault file would silently lose data, so
		// a second launch hands over to the running window instead.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.coffer.desktop",
		},
		Windows: &windows.Options{
			Theme: windows.Dark,
		},
		Mac: &mac.Options{
			TitleBar:   mac.TitleBarDefault(),
			Appearance: mac.NSAppearanceNameDarkAqua,
			About: &mac.AboutInfo{
				Title:   appName,
				Message: "A local-first password vault.\nYour vault never leaves this machine.",
			},
		},
		Linux: &linux.Options{
			ProgramName:      appName,
			WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand,
		},
	})
	if err != nil {
		log.Fatalf("coffer: %v", err)
	}
}
