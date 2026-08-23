# vncgate 远程桌面网关

基于 noVNC 的轻量 Web 远程桌面网关。内置 Web 服务与 VNC 反代，浏览器中即可访问 Linux 桌面（VNC 服务）。

## 功能特性

- **纯 Web 访问**：无需安装客户端，浏览器打开页面即用，登录后进入无工具条的全屏 noVNC 界面
- **系统登录认证**：使用 Linux 系统账号（非 root 用户）与密码登录，基于 shadow 口令验证
- **UDS 优先 / TCP 回退**：默认连接 `/tmp/vnc.sock`（Unix socket），不可用时自动回退到 TCP `:5900`
- **单二进制静态发布**：前端资源通过 `go:embed` 内嵌，无需额外部署静态文件
- **多架构交叉编译**：支持 amd64 / arm64 / arm，可选 UPX 压缩

## 编译

```bash
make all          # 交叉编译 amd64/arm64/arm，输出到 bin/
make upx          # 编译并 UPX 压缩（需安装 upx）
make clean
```

产物输出到 `bin/`：

- `bin/vncgate-linux-amd64`
- `bin/vncgate-linux-arm64`
- `bin/vncgate-linux-arm`

## 运行

需要 root 权限（用于读取 `/etc/shadow` 做口令校验）。

```bash
./bin/vncgate-linux-amd64 -web 8080 -sock /tmp/vnc.sock -port 5900
```

| 参数 | 默认值 | 说明 |
| ---- | ---- | ---- |
| `-web` | `8080` | Web 监听端口 |
| `-sock` | `/tmp/vnc.sock` | VNC Unix socket 路径（优先） |
| `-port` | `5900` | VNC TCP 端口（回退） |
| `-l` | `INFO` | 日志等级（DEBUG/INFO/WARN/ERROR） |

程序运行后会创建 `/tmp/vncgate.lock` 文件锁，防止多实例运行。

### 反向代理示例（Nginx + TLS）

建议通过 Nginx 等代理启用 HTTPS/WebSocket 转发：

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
```

## 登录与访问

- 未登录访问任意页面跳转 `/login`
- 公开页面：`/login`、`/api/login`、`/api/logout`、`/favicon.ico`
- 认证页面：`/`（远程桌面）、`/vnc.html`、`/websockify`（WebSocket）、静态资源 `/app/` `/core/` `/vendor/`

默认启动后打开 `http://<主机>:8080/` 即进入登录页。

## 项目结构

```
vncgate/
├── main.go          # 入口与版本信息
├── config.go        # 命令行参数解析
├── server.go        # 路由注册 / Web 服务
├── auth.go          # 系统登录认证与会话
├── vnc.go           # UDS/TCP 反代 + WebSocket 中继
├── logx.go          # 日志封装
├── Makefile
├── LICENSE          # Apache-2.0（本项目自研代码）
├── THIRD_PARTY_LICENSES
├── templates/
│   ├── login.html   # 登录页
│   ├── novnc.html   # 远程桌面页（内嵌 iframe）
│   └── novnc/       # noVNC 前端资源（MPL-2.0，见其 LICENSE.txt）
```

## 许可证与商标声明

- 本项目自研代码以 **Apache License 2.0** 发布，见 `LICENSE`
- 内置的 noVNC 前端（`templates/novnc/`）以 **Mozilla Public License 2.0** 发布，见 `templates/novnc/LICENSE.txt`
- 依赖组件许可详见 `THIRD_PARTY_LICENSES`

> **商标声明**：noVNC 是 noVNC 项目所有者的注册商标，本项目为独立开发，与 noVNC 官方组织无隶属或背书关系。仅使用该名称描述本项目实现的兼容功能。
