# 安全策略

## 安全模型

WebMux 以启动服务的系统用户身份执行任意命令，**等同于把一个 shell 暴露到网络上**。它不是进程沙箱，也不提供多用户隔离。安全依赖于网络隔离：

- 默认只监听 `127.0.0.1:7681`。远程访问请绑定 Tailscale / WireGuard 等可信 VPN 地址，并用防火墙限制可访问的接口。
- 设置随机的共享 `token`。监听所有接口（如 `0.0.0.0`）且未设置 token 时，服务仍会启动，但会打印警告。
- **不要把 WebMux 直接暴露到公网。**

内置的防护措施：

- 所有 API 与 WebSocket 都需要认证（启用 token 时），token 以常量时间比较。
- 带 Origin 的请求必须同源或位于 `allowed_origins` 中，防止其他网站通过浏览器发起连接。
- 终端输入输出不会写入日志；配置接口不返回 token。
- 响应头包含 `X-Content-Type-Options: nosniff` 与 `Referrer-Policy: no-referrer`。

其他注意事项：

- WebMux 不内置 TLS；需要 HTTPS 时请使用 `tailscale serve` 或反向代理。
- token 保存在浏览器 localStorage 中，只在可信设备上登录。
- 登记的 Git 仓库必须可信：Git 会执行仓库配置中的 hooks 与 filters。
- 关闭会话会清理其进程树，但完全脱离终端会话的自守护进程可能不受控制。

## 支持的版本

只有最新发布版本会接收安全修复。

## 报告漏洞

请**不要**通过公开 Issue 报告安全漏洞。请使用 GitHub 的 [私密漏洞报告](https://github.com/mzdluo123/webmux/security/advisories/new) 功能，并提供：

- 受影响的版本与平台；
- 复现步骤或概念验证；
- 可能的影响范围。

我们会尽快确认并处理。
