# Cross-platform SOCKS5 Bridge

跨平台 SOCKS5 工具：桌面 GUI 使用 Go/Fyne 实现，是独立原生窗口；不使用浏览器或 PowerShell。项目使用 Go 源码和 POSIX `sh` 脚本，不包含 `.bat` 启动器。

## 代理模式

- **单应用 SOCKS5**：只代理配置了 SOCKS5 的应用。默认监听 `127.0.0.1:1080`。
- **系统代理**：启动本地 HTTP/CONNECT→SOCKS5 桥接，并设置 Windows 或 GNOME 系统代理；停止时恢复之前的设置。只影响遵循系统代理的应用。
- **TUN 全流量代理**：接管 IPv4/IPv6 默认路由，通过上游 SOCKS5 转发 TCP/UDP。Windows 需管理员权限，Linux 需 root；上游必须支持 SOCKS5 UDP ASSOCIATE。

在 GUI 中选择模式，填入上游地址（例如 `socks5h://user:password@proxy.example.com:1080`），点击“启动代理”。单应用模式下，目标应用使用 `127.0.0.1:1080`。停止代理或正常关闭 GUI 会尝试恢复系统代理/路由。

系统代理模式支持 Windows 和 GNOME Linux；其他 Linux 桌面环境可用 TUN 模式。GUI 的清除按钮只清除 Windows 当前用户的手动系统代理。

上游 URL 支持 `socks5://[用户名:密码@]主机:端口` 和 `socks5h://...`。带特殊字符的凭据需进行 URL 编码。单应用代理支持 TCP `CONNECT`；TUN 模式的 UDP 能力取决于上游 SOCKS5 服务器。

## 启动

- Windows：双击构建出的 `win-socks5-bridge-native.exe`，或在 Git Bash 中运行 `./start.sh`。
- Linux：运行 `./start.sh`；无已构建二进制时会调用 `go run .`。
- 命令行：运行 `./start-cli.sh` 并输入上游地址，或设置 `SOCKS5_UPSTREAM`。

```sh
SOCKS5_UPSTREAM='socks5://user:pass@proxy.example.com:1080' ./start-cli.sh
```

## 构建

需要 Go 1.23.1+。首次构建会下载 Fyne 与 tun2socks 模块。Fyne 使用 CGO：Windows 构建需要 GCC/MinGW-w64 工具链；Linux 需要 GCC、pkg-config 及 X11/OpenGL 开发库（Debian/Ubuntu：`gcc pkg-config libgl1-mesa-dev xorg-dev`）。

```sh
./build.sh windows amd64
./build.sh linux amd64
```

Windows 运行脚本时可使用 Git Bash。Linux 交叉构建 Windows 需安装 MinGW-w64，可通过 `MINGW_GCC=x86_64-w64-mingw32-gcc ./build.sh windows amd64` 指定编译器。

TUN 会修改系统路由；如果启动失败，GUI 会显示错误。请先确认具有所需权限，并确保上游 SOCKS5 支持 UDP ASSOCIATE。
