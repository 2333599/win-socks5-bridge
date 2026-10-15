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
	mode := widget.NewSelect([]string{modeApplication, modeSystem, modeTUN}, nil)
	mode.SetSelected(modeApplication)
	listen := widget.NewEntry()
	listen.SetText("127.0.0.1:1080")
	status := widget.NewLabel("状态：未启动")
	logView := widget.NewMultiLineEntry()
	logView.SetPlaceHolder("运行状态和错误会显示在这里")
	logView.Disable()

	manager := &modeManager{}
	start := widget.NewButton("启动代理", nil)
	stop := widget.NewButton("停止代理", nil)
	stop.Disable()
	clear := widget.NewButton("清除 Windows 手动代理", nil)
	if runtime.GOOS != "windows" {
		clear.SetText("Windows 清除手动代理")
		clear.Disable()
	}
	mode.OnChanged = func(value string) {
		switch value {
		case modeSystem:
			if listen.Text == "127.0.0.1:1080" || listen.Text == "" {
				listen.SetText("127.0.0.1:8080")
			}
			listen.Enable()
		case modeApplication:
			if listen.Text == "127.0.0.1:8080" || listen.Text == "" {
				listen.SetText("127.0.0.1:1080")
			}
			listen.Enable()
		case modeTUN:
			listen.Disable()
		}
	}

	launch := func() {
		address := strings.TrimSpace(listen.Text)
		if address == "" {
			address = "127.0.0.1:1080"
			listen.SetText(address)
		}
		if err := manager.Start(mode.Selected, address, strings.TrimSpace(upstream.Text)); err != nil {
			dialog.ShowError(err, w)
			appendLog(logView, "启动失败："+err.Error())
			return
		}
		activeMode, address := manager.Status()
		status.SetText("状态：" + activeMode + "运行中（" + address + "）")
		appendLog(logView, activeMode+"已启动（"+address+"）")
		start.Disable()
		stop.Enable()
	}
	start.OnTapped = func() {
		if mode.Selected == modeTUN {
			dialog.ShowConfirm("启动 TUN 全流量代理", "此模式会接管 IPv4/IPv6 默认路由，需要 Windows 管理员权限或 Linux root。请确认上游 SOCKS5 支持 UDP；停止时程序会恢复路由。继续？", func(ok bool) {
				if ok {
					launch()
				}
			}, w)
			return
		}
		launch()
	}

	stop.OnTapped = func() {
		if err := manager.Stop(); err != nil {
			dialog.ShowError(err, w)
			appendLog(logView, "停止时恢复系统网络设置失败："+err.Error())
		}
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
		widget.NewLabel("代理模式"),
		mode,
		widget.NewLabel("上游 SOCKS5 地址（支持 socks5:// 和 socks5h://，可选用户名/密码）"),
		upstream,
		widget.NewLabel("本地监听地址"),
		listen,
		container.NewHBox(start, stop, clear),
		status,
		widget.NewLabel("日志"),
		container.NewScroll(logView),
		widget.NewLabel("单应用模式：目标应用设置 SOCKS5 127.0.0.1:1080；系统代理模式：自动设置 Windows/GNOME 系统 HTTP(S) 代理；TUN 模式：需要管理员权限并接管系统路由。"),
	))
	w.SetOnClosed(func() { _ = manager.Stop() })
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
