package main

import (
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// runGUI opens a native desktop window (no browser or external script).
func runGUI() error {
	a := app.New()
	w := a.NewWindow("SOCKS5 代理")
	w.Resize(fyne.NewSize(680, 480))

	upstream := widget.NewEntry()
	upstream.SetPlaceHolder("socks5h://user:password@proxy.example.com:1080")
	listen := widget.NewEntry()
	listen.SetText("127.0.0.1:1080")
	status := widget.NewLabel("状态：未启动")
	logView := widget.NewMultiLineEntry()
	logView.SetPlaceHolder("运行状态和错误会显示在这里")
	logView.Disable()

	manager := newProxyManager()
	start := widget.NewButton("启动代理", nil)
	stop := widget.NewButton("停止代理", nil)
	stop.Disable()
	clear := widget.NewButton("清除 Windows 手动代理", nil)
	if runtime.GOOS != "windows" {
		clear.Disable()
	}

	start.OnTapped = func() {
		address := strings.TrimSpace(listen.Text)
		if address == "" {
			address = "127.0.0.1:1080"
			listen.SetText(address)
		}
		if err := manager.Start(address, strings.TrimSpace(upstream.Text)); err != nil {
			dialog.ShowError(err, w)
			appendLog(logView, "启动失败："+err.Error())
			return
		}
		status.SetText("状态：运行中（" + manager.Address() + "）")
		appendLog(logView, "代理已启动，监听 "+manager.Address())
		start.Disable()
		stop.Enable()
	}

	stop.OnTapped = func() {
		manager.Stop()
		status.SetText("状态：已停止")
		appendLog(logView, "代理已停止")
		start.Enable()
		stop.Disable()
	}

	clear.OnTapped = func() {
		dialog.ShowConfirm("清除系统代理", "清除当前 Windows 用户的手动系统代理设置？", func(ok bool) {
			if !ok {
				return
			}
			if err := ClearProxy(); err != nil {
				dialog.ShowError(err, w)
				appendLog(logView, "清除失败："+err.Error())
				return
			}
			appendLog(logView, "Windows 手动系统代理已清除")
		}, w)
	}

	w.SetContent(container.NewVBox(
		widget.NewLabelWithStyle("本机 SOCKS5 代理", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("上游 SOCKS5 地址（支持 socks5:// 和 socks5h://，可选用户名/密码）"),
		upstream,
		widget.NewLabel("本地监听地址"),
		listen,
		container.NewHBox(start, stop, clear),
		status,
		widget.NewLabel("日志"),
		container.NewScroll(logView),
		widget.NewLabel("启动后，在目标应用中设置 SOCKS5 代理为 127.0.0.1:1080。程序不会自动修改应用或系统代理。"),
	))
	w.SetOnClosed(func() { manager.Stop() })
	w.ShowAndRun()
	return nil
}

func appendLog(view *widget.Entry, message string) {
	if view.Text == "" {
		view.SetText(message)
	} else {
		view.SetText(view.Text + "\n" + message)
	}
}
