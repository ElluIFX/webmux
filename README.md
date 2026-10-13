# WebMux

单用户、单进程、浏览器可重连的持久化 Web 终端。目标与协议见 [WebMux.md](WebMux.md)。仅部署于 localhost、Tailscale 或 WireGuard 内网；进程重启不会保留会话。

## 核心生命周期

Session 持有 PTY 和默认 2 MiB 字节环形缓冲，连接只是观察者。附着时原子捕获快照并注册实时订阅，避免快照与续流之间漏字节。慢客户端断开后可重新回放。子进程退出前读完最终输出；关闭向进程组发送 SIGHUP，5 秒后升级 SIGKILL。已退出的会话仍保留缓冲，直到手动移除。

## 开发检查

```sh
go test -race ./...
```

前端包管理使用 pnpm，不使用 npm 安装依赖。

## API 与认证

REST 与二进制 WebSocket 帧遵循设计文档第 6 节。`GET /api/config` 仅补充返回 `{defaultCwd}`，让浏览器读取配置默认目录，不暴露 token。HTTP 使用 Bearer 或 `webmux_token` Cookie；WebSocket 也支持 query token。静态登录页面公开，所有 API / WebSocket 需要认证。浏览器跨域 API 与 WebSocket 请求均校验 Origin；反向代理应保持 Host，HTTPS 代理需把公开 Origin 加入 `allowed_origins`。

开发要求 Go 1.23+（当前 websocket 依赖的最低版本）；服务默认监听 `127.0.0.1:7681`。启动示例：

```sh
go build -o bin/webmux ./cmd/webmux
./bin/webmux --listen 127.0.0.1:7681 --token YOUR_TOKEN
curl -H 'Authorization: Bearer YOUR_TOKEN' http://127.0.0.1:7681/api/health
```

## 前端开发

```sh
cd web
pnpm install --frozen-lockfile
pnpm build
pnpm dev
```

Vite 使用 `127.0.0.1:5173`，代理到 `127.0.0.1:7681`。开发配置需设置 `allowed_origins = ["http://127.0.0.1:5173"]`，否则带 Origin 的写请求会被服务端安全校验拒绝。生产构建不需要此开发 Origin。

前端使用 Preact、TypeScript、xterm 5；WebGL 不可用或丢失上下文时回退 xterm 内置 DOM 渲染器（允许的依赖列表没有 canvas addon）。欢迎页通过 WebSocket 更新，终端重连清屏后全量回放。追加参数按 argv 引号规则拆分，不通过 shell 执行。

## 标题与状态

OSC 0/2 title 支持 BEL / ST 结束符及跨块拼接；创建时指定名称会覆盖自动标题。预览去除分块 ANSI / OSC 序列并保留最多 120 字符。状态每秒评估，列表更新最多每 500 ms 一次，预览变化最多每 2 秒一次。

最近 2 秒有输出为“运行中”；之后提示符匹配或 Linux 进程树叶子全部 sleeping 且无 running 进程时为“可能在等你”；2–30 秒无法判断时同样提示查看，超过配置空闲阈值且无等待特征为“空闲”。非 Linux 只使用提示符与时间。此启发式会把 sleep / 网络等待判为等待输入，也可能把持续重绘的 TUI 判为运行中；不会驱动自动动作。

## 手机与平板

终端使用 VisualViewport 跟随键盘高度与安全区域，支持横竖屏 fit、触摸历史滚动、双指字体缩放（12–20 px）、可折叠软键栏和一次性 Ctrl / Alt 修饰键。终端 textarea 禁用自动大写、拼写检查与自动纠正。iOS / Android 可添加到主屏幕；HTTP 私网下不提供离线缓存或 service worker。

HTTP 内网通常无法使用系统 Clipboard API：点“粘贴”打开输入框，粘贴后点“发送”。通过 xterm 的 paste API 遵循应用启用的 bracketed paste 模式：Bash / Claude 开启该模式时，多行内容不会被逐行误提交。桌面可用 Ctrl+Shift+C/V；复制全部缓冲在剪贴板不可用时提供可选中文本。

切后台停止写入 xterm，但保持连接和服务端输出；回前台主动重新附着并回放，避免 iOS 半失效连接仍显示 OPEN。指数退避 0.5–8 秒，恢复网络时立即尝试。真正的 iOS 输入法、VPN 切换与锁屏恢复仍需在实际设备上验收，桌面设备模拟不能证明这些系统行为。
