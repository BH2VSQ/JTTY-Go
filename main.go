package main

import (
	"context"
	"embed"
	"log"

	"github.com/BH2VSQ/jtty-go/internal/app"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist all:bin
var assets embed.FS

type wailsSink struct{ ctx context.Context }

func (s wailsSink) Emit(event string, payload any) { wailsruntime.EventsEmit(s.ctx, event, payload) }

const minMainWindowWidth = 1080

func main() {
	application := app.New()
	application.SetEmbeddedAssets(assets)
	err := wails.Run(&options.App{Title: "JTTY-Go", Width: 1360, Height: 820, MinWidth: minMainWindowWidth, MinHeight: 680, BackgroundColour: &options.RGBA{R: 244, G: 247, B: 250, A: 255}, AssetServer: &assetserver.Options{Assets: assets}, Bind: []interface{}{application}, OnStartup: func(ctx context.Context) { application.SetEventSink(wailsSink{ctx: ctx}); application.Startup(ctx) }, OnDomReady: application.DomReady, OnBeforeClose: application.BeforeClose, Windows: &windows.Options{WebviewIsTransparent: false}, Mac: &mac.Options{TitleBar: mac.TitleBarHiddenInset()}})
	if err != nil {
		log.Fatal(err)
	}
}
