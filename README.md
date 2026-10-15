# Cross-platform SOCKS5 Bridge

跨平台 SOCKS5 工具：在本机开启 SOCKS5 入口，并通过另一个 SOCKS5 服务器转发 TCP 流量。桌面 GUI 使用 Go/Fyne 编写，是独立原生窗口；不使用浏览器或 PowerShell。代理与界面源码可构建到 Windows 和 Linux。

## 使用

- Windows：双击 `win-socks5-bridge-native.exe`，或双击 `start.bat`。
- Linux：运行 `./socks5-bridge-linux`。
- 在 GUI 填入上游地址，如 `socks5h://user:password@proxy.example.com:1080`，选择本地监听地址并点击“启动代理”。
- 在目标应用中配置 SOCKS5 `127.0.0.1:1080`。GUI 不会自动修改浏览器或系统代理。
- 点击“停止代理”或关闭 GUI 可停止代理。

GUI 中清除系统代理按钮仅在 Windows 可用：会清除当前用户的手动代理设置，不修改自动检测或 PAC 脚本。Linux 目前不修改桌面环境代理设置。

上游 URL 支持 `socks5://[用户名:密码@]主机:端口` 和 `socks5h://...`。带特殊字符的凭据需进行 URL 编码。代理支持 SOCKS5 `CONNECT`（TCP），不支持 UDP ASSOCIATE 或透明代理。

## 命令行

Windows 命令行版本是 `win-socks5-bridge-cli.exe`；Linux 可使用 `socks5-bridge-linux`：

```sh
./socks5-bridge-linux -listen 127.0.0.1:1080 -upstream 'socks5://user:pass@proxy.example.com:1080'
```

## 构建

需要 Go 1.20+。首次构建需下载 Fyne Go 模块。Fyne 桌面驱动使用 CGO：Windows 构建需要可用的 GCC/MinGW-w64 工具链，Linux 构建需要 GCC 及 X11/OpenGL 开发库。

### Windows

```powershell
$env:GOOS = 'windows'; $env:GOARCH = 'amd64'
# 双击启动的原生 GUI 版本（隐藏控制台）
go build -trimpath -ldflags='-s -w -H=windowsgui' -o win-socks5-bridge-native.exe .
# 命令行版本
go build -trimpath -ldflags='-s -w' -o win-socks5-bridge-cli.exe .
```

### Linux

Fyne 的 Linux 桌面驱动需要 C 编译器、pkg-config、X11/OpenGL 开发库（Debian/Ubuntu：`gcc pkg-config libgl1-mesa-dev xorg-dev`）。

```sh
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o socks5-bridge-linux .
```
