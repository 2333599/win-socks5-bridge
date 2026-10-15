# Cross-platform SOCKS5 Bridge

跨平台 SOCKS5 工具：在本机开启 SOCKS5 入口，并通过另一个 SOCKS5 服务器转发 TCP 流量。桌面 GUI 使用 Go/Fyne 编写，是独立原生窗口；不使用浏览器或 PowerShell。项目使用 Go 源码和 POSIX `sh` 脚本，不包含 `.bat` 启动器。

## 使用

- Windows：双击构建出的 `win-socks5-bridge-native.exe`，或在 Git Bash 中运行 `./start.sh`。
- Linux：运行 `./start.sh`；需要源码运行时会调用 `go run .`。
- GUI 中填入上游地址，如 `socks5h://user:password@proxy.example.com:1080`，设置本地监听地址并点击“启动代理”。
- 在目标应用中配置 SOCKS5 `127.0.0.1:1080`。GUI 不会自动修改浏览器或系统代理。
- 点击“停止代理”或关闭 GUI 可停止代理。

GUI 中清除系统代理按钮仅在 Windows 可用：会清除当前用户的手动代理设置，不修改自动检测或 PAC 脚本。也可在 Windows Git Bash 中运行 `./clear-proxy.sh`。Linux 目前不修改桌面环境代理设置。

上游 URL 支持 `socks5://[用户名:密码@]主机:端口` 和 `socks5h://...`。带特殊字符的凭据需进行 URL 编码。代理支持 SOCKS5 `CONNECT`（TCP），不支持 UDP ASSOCIATE 或透明代理。

## 命令行

运行 `./start-cli.sh` 并输入上游地址，或通过环境变量提供地址：

```sh
SOCKS5_UPSTREAM='socks5://user:pass@proxy.example.com:1080' ./start-cli.sh
```

也可以直接使用 Go 程序参数：

```sh
go run . -listen 127.0.0.1:1080 -upstream 'socks5://user:pass@proxy.example.com:1080'
```

## 构建

需要 Go 1.20+。首次构建会下载 Fyne Go 模块。Fyne 使用 CGO：Windows 构建需要 GCC/MinGW-w64 工具链，Linux 需要 GCC、pkg-config 及 X11/OpenGL 开发库（Debian/Ubuntu：`gcc pkg-config libgl1-mesa-dev xorg-dev`）。

使用统一的 POSIX shell 构建脚本：

```sh
./build.sh windows amd64
./build.sh linux amd64
```

Windows 运行 `build.sh` 时可使用 Git Bash；交叉构建需配置对应平台的 C 编译器。
