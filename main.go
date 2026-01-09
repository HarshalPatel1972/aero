// MIT License
//
// Copyright (c) 2026 Project AERO Contributors
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create application instance
	app := NewApp()

	// Run Wails application
	err := wails.Run(&options.App{
		Title:     "AERO",
		Width:     400,
		Height:    600,
		MinWidth:  400,
		MinHeight: 600,
		MaxWidth:  400,
		MaxHeight: 600,

		// Frameless for custom title bar
		Frameless:         true,
		DisableResize:     true,
		StartHidden:       false,
		HideWindowOnClose: false,

		// Asset server configuration
		AssetServer: &assetserver.Options{
			Assets: assets,
		},

		// Background color (fallback when transparency not available)
		BackgroundColour: &options.RGBA{R: 26, G: 26, B: 26, A: 255},

		// Lifecycle callbacks
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,

		// Bind Go methods to frontend
		Bind: []interface{}{
			app,
		},

		// Windows-specific options
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			BackdropType:         windows.Mica,
			DisableWindowIcon:    false,
			Theme:                windows.Dark,
			CustomTheme: &windows.ThemeSettings{
				DarkModeTitleBar:   windows.RGB(26, 26, 26),
				DarkModeTitleText:  windows.RGB(255, 255, 255),
				DarkModeBorder:     windows.RGB(51, 51, 51),
				LightModeTitleBar:  windows.RGB(26, 26, 26),
				LightModeTitleText: windows.RGB(255, 255, 255),
				LightModeBorder:    windows.RGB(51, 51, 51),
			},
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
