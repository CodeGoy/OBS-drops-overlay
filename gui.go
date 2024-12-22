//go:build windows

package main

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

func systray() {
	a := app.NewWithID("codegoy.obs.overlay")
	version = a.Metadata().Version
	a.SetIcon(fyne.NewStaticResource("icon", icon))
	if desk, ok := a.(desktop.App); ok {
		a.SetIcon(fyne.NewStaticResource("icon", icon))
		w := a.NewWindow(fmt.Sprintf("OBS-drops-overlay v%s", version))
		w.SetFixedSize(true)
		m := fyne.NewMenu("links",
			fyne.NewMenuItem("Links", func() {
				w.Show()
			}),
		)
		desk.SetSystemTrayMenu(m)
		w.SetContent(widget.NewRichTextFromMarkdown(fmt.Sprintf(`# Links
* [%s](%s)
* [%s](%s)`, controlLink, controlLink, overlayLink, overlayLink)))
		w.SetCloseIntercept(func() {
			w.Hide()
		})
		a.Run()
	}
}
