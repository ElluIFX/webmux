# Windows 原生支持

## 运行要求

- Windows 10 1809 / Windows Server 2019 或更新版本，具有 ConPTY 支持。推荐 Windows 11。
- PowerShell（系统自带的 Windows PowerShell 或自行安装的 PowerShell 7）或显式配置的其他 shell。
- 使用仓库、Diff 与 worktree 功能时，需要 Git for Windows，且 `git.exe` 位于 WebMux 进程的 PATH 中。
- 不需要 WSL、MSYS2、tmux、Node.js 或 Go 来运行已构建的单文件程序。源码构建需要 Go 1.24+、Node.js 24 与 pnpm 12.11.2。

## 构建及运行

在仓库根目录使用 PowerShell，无需 GNU Make：

```powershell
pnpm --dir web install --frozen-lockfile
pnpm --dir web build
$env:CGO_ENABLED = "0"
go build -trimpath -o bin/webmux.exe ./cmd/webmux
.\bin\webmux.exe --listen 127.0.0.1:7681
```

浏览器打开 <http://127.0.0.1:7681>。保持控制台运行；关闭浏览器不会结束终端，但关闭服务器会结束会话。WebMux 不是 Windows 服务，本次支持不包含服务安装器、自动启动设置或代码签名。

仅监听 loopback 或可信 VPN，并设置 token。不要直接将交互 shell 暴露到公网。程序以启动它的 Windows 用户权限运行，既不是权限隔离层，也不是沙箱。

## 配置和路径

默认配置位于 `%APPDATA%\webmux\config.toml`，仓库清单位于 `%LOCALAPPDATA%\webmux`。默认 shell 为系统 Windows PowerShell，以 `-NoLogo -NoExit` 启动；显式选择 `pwsh.exe` 或 `cmd.exe` 时使用对应启动参数。

可用 `--config C:\path\config.toml` 指定配置文件。TOML 中的 Windows 路径推荐用单引号，避免反斜杠被当成转义：

```toml
default_cwd = 'C:\Users\Alice\projects'
state_dir = 'C:\Users\Alice\AppData\Local\webmux\state'
shell = 'C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe'

[[presets]]
name = 'shell'
label = 'PowerShell'
cmd = ['powershell.exe', '-NoLogo', '-NoProfile']
```

显式的 `cmd` 是 argv 数组，不会经过 shell 字符串展开。运行脚本时，应显式选择解释器及参数，例如 PowerShell 的 `-File`。命令的可执行文件或脚本是否可用，仍取决于服务器用户的 PATH、执行策略与 CLI 自身的 Windows 支持。

### npm 与批处理启动器

`.cmd` / `.bat` 不能直接作为原生终端的可执行文件。Windows 上某些 npm 全局命令（例如 Codex 或 omp）会解析为 `.cmd` 启动器；因此内置预设可能需要在本机显式配置。WebMux 会给出明确错误，不会自动把任意 argv 拼成 `cmd /c`，以免改变参数边界或引入命令注入。

优先使用 CLI 官方的原生 `.exe`；若 CLI 是 Node 程序，可把实际安装位置的 `node.exe` 与 JavaScript 入口分别写进 argv：

```toml
[[presets]]
name = 'my-agent'
label = 'Node agent'
cmd = ['C:\Program Files\nodejs\node.exe', 'C:\path\to\agent\cli.js']
allow_args = true
```

这里的入口路径需要替换为该 CLI 实际安装的文件，不是通用 npm 路径。也可先创建 PowerShell 会话，再像平时一样手动运行已安装的命令；此时由用户选择的 shell 处理其命令语义。

工作区代码浏览继续使用相对路径与禁止越界的只读访问。Windows 文件系统的 junction/reparse point、设备路径及 NTFS alternate data streams 不应成为逃逸工作区的途径。不要通过放宽这一安全边界来兼容特殊路径。

上传临时目录使用创建时设置的、仅当前用户可访问并向文件继承的受保护 DACL；请使用支持 Windows ACL 的本地文件系统。

状态目录保存仓库清单，不保存终端会话；一个状态目录只能由一个 WebMux 实例持有。Windows 的访问控制使用目录继承的 ACL，而不是 Unix mode 位，因此应将状态目录放在当前用户独占的目录，不要放到共享可写目录。

## 生命周期和兼容性

Windows 使用 ConPTY 传递终端字节，使用 Job Object 管理其拥有的进程树。输入取消、终端尺寸调整、输出回放和 WebSocket 协议沿用共享实现。Linux/macOS 保留原来的 PTY 及进程管理后端；平台相关代码通过小范围 helper 和构建标签分离，减少后续合入上游的冲突。

Windows 没有 Unix 的前台进程组和 `/proc`，因此依赖 Unix 进程状态的“等待输入”推断可能不同；终端输出、提示符及 agent 协议仍可提供状态信息。不同 shell 的 quoting 和快捷键语义也可能不同。Windows ARM64 可交叉构建，但 amd64 原生 CI 不等于 ARM64 运行验证。

## 测试

GitHub Actions 的 Windows 作业执行原生 Go 测试，包括 ConPTY、进程清理和文件系统边界测试，而不是只交叉编译。它同时运行前端单元、浏览器组件测试及 Windows 原生端到端 smoke 测试。Linux/macOS 保留完整现有浏览器端到端套件。

本机运行：

```powershell
pnpm --dir web build
go test -timeout 10m ./...
go vet ./...
pnpm --dir web test:unit
pnpm --dir web exec playwright install chromium
pnpm --dir web test:browser
pnpm --dir web exec playwright test --config playwright.config.ts windows.spec.ts
```

安装兼容的 C 编译器后可运行 `go test -race -timeout 10m ./...`；Windows CI 的宿主提供所需工具链。构建成功本身不证明原生 ConPTY 或文件系统行为通过，需查看对应平台测试结果。
