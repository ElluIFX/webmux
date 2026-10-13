# WebMux

单用户、单进程、可重连的 Web 终端。浏览器关闭、切网不会结束服务端 PTY；重新附着会清屏并回放当前缓冲。用于在可信内网中操作 Bash、Claude Code、Codex CLI 等。目标与协议见 [WebMux.md](WebMux.md)。

**安全边界：这相当于以当前 Linux 用户身份把 shell 暴露到网络。不要开放公网；默认只绑定 `127.0.0.1:7681`。使用 Tailscale / WireGuard 地址并设置共享 token。服务重启结束所有会话，不落盘。**

## 从源码构建

需要 Go **1.23+**（websocket 依赖的最低版本）、Node.js **24 LTS**、pnpm **12.11.2** 和 GNU Make。运行端只需 Linux amd64 / arm64、可执行 shell 与所选 agent；不需要 Node.js、Go 或 tmux。

```sh
corepack enable
corepack prepare pnpm@12.11.2 --activate
make build
./bin/webmux version
./bin/webmux --listen 127.0.0.1:7681
```

若 Node 安装未附带 Corepack，可按 pnpm 官方安装方式安装 pnpm。前端所有依赖由 `web/pnpm-lock.yaml` 锁定，Makefile 始终使用 `pnpm install --frozen-lockfile`，只允许 esbuild 的必要安装脚本。

- `make web`：构建前端到 `web/dist`。
- `make build`：构建内嵌静态资源的单二进制 `bin/webmux`。
- `make release VERSION=0.1.0`：生成 `bin/webmux-linux-amd64`、`bin/webmux-linux-arm64`，CGO 禁用。
- `make install`：构建并安装到 `~/.local/bin/webmux`。
- `make test`：Go race 测试与前端控制键行为测试。
- `make check`：前端严格 TypeScript 构建、`go vet`、golangci-lint 默认规则。需先将 `golangci-lint` 放到 PATH。

macOS 可编译运行，`/proc` 探测在非 Linux 平台返回 false；发布目标为 Linux。未构建前端时 `.gitkeep` 让 `go test` 能工作，但该状态的 HTTP 页面明确返回 503，不能当作可部署产品。

## 配置

```sh
mkdir -p ~/.config/webmux
cp deploy/config.example.toml ~/.config/webmux/config.toml
chmod 600 ~/.config/webmux/config.toml
```

编辑配置，将 `listen` 改为自己的 Tailscale / WireGuard 地址，例如 `100.64.0.5:7681`，`token` 改为自己的随机共享密钥。默认配置文件为 `~/.config/webmux/config.toml`：不存在时用默认值；显式 `--config` 指定的文件不存在时启动失败。CLI `--listen` / `--token` 优先于 TOML，包括显式空 token。

```sh
./bin/webmux --config ~/.config/webmux/config.toml
./bin/webmux --listen 127.0.0.1:7681 --token YOUR_TOKEN
curl -H 'Authorization: Bearer YOUR_TOKEN' http://127.0.0.1:7681/api/health
```

| 配置 | 默认 | 含义 |
| --- | --- | --- |
| `listen` | `127.0.0.1:7681` | HTTP / WebSocket 监听地址 |
| `token` | 空 | 可选共享 token，空表示无认证 |
| `scrollback_bytes` | `2097152` | 每会话保留最新 2 MiB 原始终端字节，不是精确行数 |
| `default_cwd` | `~` | 新建会话默认目录，支持 `~` 展开 |
| `shell` | `$SHELL` 或 `/bin/bash` | 默认 shell，以 `-l -i` 启动 |
| `idle_after_seconds` | `30` | 无等待特征时的空闲阈值 |
| `prompt_patterns` | 常见提示符 / 权限确认 | 可自定义的 Go 正则表达式列表 |
| `allowed_origins` | `[]` | 附加允许的完整 Origin，例如 `https://myhost.example` |
| `presets` | shell / claude / codex | argv 数组及 `allow_args`，配置列表替换默认列表 |

预设不会自动安装 agent；`which claude`、`which codex` 必须能找到实际程序。追加参数按引号与反斜杠规则拆成 argv，不经 shell 求值。要更改 shell 的启动方式，可为 shell 预设显式设置 `cmd = ["/bin/zsh", "-l", "-i"]`。

## systemd 用户服务

```sh
make install
mkdir -p ~/.config/systemd/user
cp deploy/webmux.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now webmux
systemctl --user status webmux
journalctl --user -u webmux -f
```

先按上节创建配置。`deploy/webmux.service` 的 PATH 包含 `~/.local/bin`、pnpm / npm 全局 bin；若 agent 在 nvm、其他自定义目录中，通过用户 drop-in 增加真实路径：

```sh
which claude
which codex
systemctl --user edit webmux
```

```ini
[Service]
Environment=PATH=/home/YOU/.local/bin:/home/YOU/.nvm/versions/node/v24.20.0/bin:/usr/local/bin:/usr/bin:/bin
```

重载并重启服务。用户未登录时仍需运行，可执行 `loginctl enable-linger "$USER"`（可能需要管理员授权）。服务使用 `KillMode=mixed`，先让 WebMux 正常清理会话，再由 systemd 在超时后清理整个 cgroup。终止 / 重启服务会永久丢失会话与缓冲。

## Tailscale 与手机访问

1. 在服务器和手机安装 Tailscale，加入同一可信网络。
2. 绑定服务器的实际 Tailscale 地址，防火墙只允许可信 VPN 接口。
3. 手机浏览器打开 `http://<Tailscale-IP或名称>:7681`。
4. 启用 token 时输入一次，浏览器 localStorage 保存；只在可信设备上保存密钥。
5. iOS Safari / Android Chrome 可“添加到主屏幕”。

WebMux 不内置 TLS。需要安全上下文与系统剪贴板 API 时，可用 `tailscale serve` 或 Caddy 代理 HTTPS；保持请求 Host，并把公开 HTTPS Origin 加入 `allowed_origins`。不能让不可信客户端伪造同源域名绕过网络隔离。HTTP 内网下不提供离线缓存或 service worker。

## 使用

欢迎页按最近活跃时间显示会话、完整目录（视觉上中间省略）、命令、状态、预览与连接数。列表通过 WebSocket 实时更新，不轮询。新建支持目录补全、确认后创建目录、最近 5 个目录快捷选项（本地最多保存 10 个）、预设追加参数和可选固定名称。

- 点击卡片进入终端；右上角 × 二次确认后关闭或移除已退出会话。
- 终端菜单提供复制全部原始缓冲、清屏、重新同步、字体大小和关闭会话。
- 已退出会话保留退出码与最后输出，直到手动移除。
- 同一会话可多端同时连接；**最后一次 resize 的客户端决定 PTY 尺寸**。
- 默认服务端缓冲 2 MiB、xterm 历史 10,000 行；超过上限丢弃旧数据。

### 移动端

VisualViewport 跟随键盘高度与安全区域，横竖屏自动 fit；支持触摸历史滚动、双指字体缩放（12–20 px）、可折叠软键栏。Ctrl / Alt 为一次性修饰键。终端输入禁用自动大写、拼写检查与自动纠正。常用 Ctrl+C/D/Z/L/R、Esc、Tab、方向、Home/End、PgUp/PgDn 都有按钮。

HTTP 私网通常无法读 Clipboard API：点“粘贴”后在输入框粘贴并“发送”。xterm 的 paste API 遵循应用启用的 bracketed paste 模式：Bash / Claude 开启时多行文本不会被逐行误提交。桌面使用 Ctrl+Shift+C/V；复制全部在剪贴板不可用时提供可选中文本。

切后台停止写入 xterm，连接与 PTY 保持；回前台主动重新附着并回放，避免半失效连接仍显示 OPEN。指数退避 0.5–8 秒，回前台 / 恢复网络时立即尝试。真机 iOS 输入法、VPN 切换与锁屏恢复不能用桌面设备模拟证明。

### 标题与状态

OSC 0/2 title 支持 BEL / ST 与跨块拼接，创建时的手动名称优先。预览去除增量 ANSI / OSC 序列，最多 120 字符，不是完整终端仿真。Linux 进程表每秒刷新，时间与提示符状态每 500 ms 评估，为 3 秒等待提示留出传输余量；列表更新每会话最多 500 ms 一次，预览变化最多 2 秒一次。

- 最近 2 秒有输出：运行中。
- 之后匹配提示符，或 Linux 进程树无 running 成员、叶子全 sleeping：可能在等你。
- 2–30 秒不确定：同样提示查看；超过空闲阈值且没有等待特征：空闲。
- 子进程结束：已退出，记录真实退出码；信号退出记为 `128 + signal`。

启发式可能把 sleep / 网络等待判为等待，也可能把不断重绘的 TUI 判为运行中。状态只展示，不自动执行动作。

## 协议与实现

REST 与二进制 WebSocket 帧遵循设计文档第 6 节。补充只读 `GET /api/config` 返回 `{defaultCwd}`，不暴露 token。HTTP 接受 Bearer 或 `webmux_token` Cookie；WebSocket 也可用 query token。静态登录页面公开，所有 API / WebSocket 受认证保护。token 以定长摘要常量时间比较；浏览器 API / WebSocket 均校验同源或允许的 Origin。

Session 持有 PTY 与有界字节环形缓冲，连接只是观察者。快照与订阅在同一锁内切换，无回放缺口；慢客户端断开后可重连。PTY 使用 Go netpoll 支持的非阻塞描述符，断开连接能取消阻塞输入，不会关闭持久会话；每连接输入队列有上限，单次写入最长 10 秒。会话退出先清理子进程并排空 PTY，再发送退出帧。关闭先 SIGHUP，5 秒后 SIGKILL；Linux 按会话 ID、进程树与继承的 WEBMUX_SESSION_ID 找到其他 job-control 进程组，并用 subreaper 回收孤儿子进程，不因主进程先退出而取消清理。显式清掉环境再完全脱离会话的自守护程序不属于可靠可控的 PTY 子进程；本服务不是进程沙箱。不会把终端输入输出写入日志。

前端使用 Preact / TypeScript / Vite / xterm 5，无 UI 或状态库；WebGL 不可用或上下文丢失时回退 xterm 内置 DOM 渲染器。允许的依赖没有 canvas addon，未另加依赖。

## 开发

```sh
pnpm --dir web install --frozen-lockfile
pnpm --dir web dev
go test -race ./...
pnpm --dir web test
```

Vite 使用 `127.0.0.1:5173`，代理到 `127.0.0.1:7681`。**开发配置需** `allowed_origins = ["http://127.0.0.1:5173"]`，否则带 Origin 的写请求会被拒绝。生产不需要开发 Origin。

## FAQ

**创建失败，找不到 claude / codex？** 检查服务用户的 PATH；systemd 不会执行你的交互 shell 初始化文件。预设显示实际 argv，程序缺失不会被误报为目录不存在。

**断线后旧内容有一点乱码？** 字节环形缓冲可能从 UTF-8 / 转义序列中间开始。前端全量重放，首行偶发残片在设计允许范围内。

**关闭手机页面是否关闭会话？** 不会。只有子进程退出、手动关闭或服务退出会结束会话。

**两个设备的布局为何互相影响？** 两端共享同一 PTY，最新 resize 生效。不是独立终端视图或分屏。

**无 token 监听所有地址为何能启动？** 为遵循设计，不拒绝但打印醒目 WARN。请改为 VPN IP 并配置 token；默认仅 loopback。

**粘贴 / 复制没有权限？** HTTP 不是安全上下文，使用内置文本框回退或配置 HTTPS。复制全部是原始缓冲，可能包含 ANSI 转义序列。

**重启之后找不到会话？** 这是本版本的明确边界：无数据库、无守护进程分离、不与 tmux 互通。
