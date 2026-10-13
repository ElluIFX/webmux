# WebMux

单用户、单进程、浏览器可重连的持久化 Web 终端。目标与协议见 [WebMux.md](WebMux.md)。仅部署于 localhost、Tailscale 或 WireGuard 内网；进程重启不会保留会话。

## 核心生命周期

Session 持有 PTY 和默认 2 MiB 字节环形缓冲，连接只是观察者。附着时原子捕获快照并注册实时订阅，避免快照与续流之间漏字节。慢客户端断开后可重新回放。子进程退出前读完最终输出；关闭向进程组发送 SIGHUP，5 秒后升级 SIGKILL。已退出的会话仍保留缓冲，直到手动移除。

## 开发检查

```sh
go test -race ./...
```

前端包管理使用 pnpm，不使用 npm 安装依赖。
