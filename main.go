package main

import (
	"embed"
	"io/fs"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/NathanBhanji/tail-chat/internal/themes"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	// The embedded assets are under frontend/dist/. Sub into that.
	distFS, _ := fs.Sub(assets, "frontend/dist")

	err := wails.Run(&options.App{
		Title:     "tailchat",
		Width:     960,
		Height:    680,
		MinWidth:  640,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Handler: themes.Handler(distFS),
		},
		BackgroundColour: &options.RGBA{R: 15, G: 15, B: 20, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				HideTitleBar:               false,
				FullSizeContent:            true,
				UseToolbar:                 true,
				HideToolbarSeparator:       true,
			},
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "tailchat",
				Message: "End-to-end encrypted chat over Tailscale",
			},
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
