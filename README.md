# TheOnlyMirror

既然镜像源太占空间，那为什么不能用反代源呢！

但加速代理域名那么多，为什么不尝试用同一个域名，代理所有的镜像源呢？

原理为不同应用请求镜像源时的 UA/PATH/HOST/HEADER 会有相应的特征，从而确定请求的镜像源。All in one 一站式镜像聚合加速服务，提供统一入口，基于请求特征智能分流，自动路由到最匹配的源站。

如果采用 Docker+Nginx 反代方式部署，请务必打开 HTTP/2 支持。

## 快速部署

感谢 @jimyag 的 PR

```shell
docker run -d --restart=unless-stopped -p 8080:8080 ghcr.io/huangzheng2016/the-only-mirror
```

部署后访问 `http://<你的地址>:8080/` 即可打开内嵌的 Web 控制台，查看换源示例、当前镜像源与 Proxy 域名，并根据当前访问地址生成可复制的命令。

## Web 控制台

服务根路径 `/` 提供内嵌的 Vue 前端（通过 go:embed 打包进二进制），无需额外静态资源：

- **换源示例**：按场景（Docker / Linux / Python / Node / Go / GitHub）展示命令，内容根据当前访问域名自动替换
- **镜像源**：支持关键字检索，以卡片形式展示当前配置的 sources，并显示 Path / UA / Upstream 等详情
- **快捷访问**：展示 host alias 映射（当前域名 → 上游），便于配置子域名代理
- **Proxy 域名**：按协议分组展示白名单中的代理域名
- 支持日间/夜间主题切换，静态资源带 hash 并设置长期缓存

## API

- **GET /api/services**  
  返回当前配置中的镜像源、Proxy 白名单与 host alias，供前端或第三方调用。  
  响应字段：`sources`、`proxy`、`hostAliases`，以及可选的 `requestHost`。

## 已支持的生态与用法

以下命令中的 `https://example.com` 请替换为你的镜像站地址。除特别说明外，均已在本地真实链路验证（2026-10）。

### Docker Hub

```shell
tee /etc/docker/daemon.json <<-'EOF'
{
  "registry-mirrors": ["https://example.com"]
}
EOF
```

`/v2/`、`/token` 按 Docker UA 分流到 registry-1.docker.io / auth.docker.io，`www-authenticate`、`location`、`link` 响应头会改写回镜像地址；CloudFront/Cloudflare blob CDN 重定向同样改写回镜像路径。

### npm

```shell
npm config set registry https://example.com
```

packument 中的 tarball 地址会改写为镜像地址（含 chunked 无 Content-Length 的响应）。

### PyPI

```shell
pip config set global.index-url https://example.com/simple
# 或单次使用
pip install -i https://example.com/simple -r requirements.txt
```

`/simple` 页面中的 `files.pythonhosted.org` 链接会改写为镜像相对路径，由 `/packages` 源承接下载。

### PyTorch（CPU 等预编译 wheel）

```shell
pip install --index-url https://example.com/whl/cpu torch
# 或作为附加索引
pip install --extra-index-url https://example.com/whl/cpu torch
```

`/whl` 源代理 download.pytorch.org；simple 页面中的 `download-r2.pytorch.org` 绝对链接会改写为镜像相对路径，wheel 由同一源承接。

### Ubuntu / Debian / Alpine 等 apt、apk 源

```shell
# sources.list 示例（Ubuntu）
deb https://example.com/ubuntu jammy main restricted
deb https://example.com/ubuntu-security jammy-security main restricted

# Debian
deb https://example.com/debian bookworm main
```

- `/ubuntu` → archive.ubuntu.com，`/ubuntu-security` → security.ubuntu.com（自动改写为上游 `/ubuntu` 路径）。
- 注意：archive.ubuntu.com 与 security.ubuntu.com 官方仅提供 HTTP 上游，镜像服务器需要能访问这两个站点的 80 端口。

### Crates.io（sparse index）

`~/.cargo/config.toml`：

```toml
[source.crates-io]
replace-with = "mirror"

[source.mirror]
registry = "sparse+https://example.com/crates-index/"
```

sparse 索引代理 index.crates.io；`config.json` 中的 `dl` 会改写为 `https://example.com/crates`，crate 下载由 `/crates` 源代理 static.crates.io。限制：发布（publish/yank）等写操作仍直连 crates.io，不支持走镜像。

### Rustup / Rust 工具链

```shell
export RUSTUP_DIST_SERVER=https://example.com
export RUSTUP_UPDATE_ROOT=https://example.com/rustup
curl --proto '=https' --tlsv1.2 https://example.com/rustup/dist/x86_64-unknown-linux-gnu/rustup-init -o /tmp/rustup-init
```

`/dist` 源代理 static.rust-lang.org 的 channel 清单与组件包（清单内的 `url`/`xz_url` 会改写为镜像地址），`/rustup` 源承接 rustup-init 与更新根。建议配合 TLS 部署使用。

### Homebrew

```shell
export HOMEBREW_API_DOMAIN=https://example.com/formulae.brew.sh/api
export HOMEBREW_BOTTLE_DOMAIN=https://example.com/ghcr.io/v2/homebrew/core
# 可选：brew/core 的 git 远程也走镜像
export HOMEBREW_BREW_GIT_REMOTE=https://example.com/github.com/Homebrew/brew.git
export HOMEBREW_CORE_GIT_REMOTE=https://example.com/github.com/Homebrew/homebrew-core.git
```

- API JSON 由白名单代理 formulae.brew.sh；bottle 走 ghcr.io 路径代理，brew 自带的匿名 `Bearer QQ==` 头会原样透传，blob 的 307 跳转（pkg-containers.githubusercontent.com）会改写回镜像路径，已实测下载的 sha256 与 digest 一致。
- 限制：`HOMEBREW_BOTTLE_DOMAIN` 必须设置，否则 API JSON 里的 bottle 地址仍指向 ghcr.io 直连；源码编译（`brew install --build-from-source`）所需的各上游 tarball 不在代理范围内。

### GitHub

```shell
# git clone（智能 HTTP 走 /github.com 前缀）
git clone https://example.com/github.com/huangzheng2016/TheOnlyMirror.git

# 文件下载
curl -O https://example.com/github.com/huangzheng2016/TheOnlyMirror/archive/master.zip
wget https://example.com/raw.githubusercontent.com/huangzheng2016/TheOnlyMirror/main/README.md
```

也支持子域名模式（需泛解析 DNS），如 `gh.你的域名`、`ghcr.你的域名`，见 `host_aliases` 配置。

### Go

```shell
export GOPROXY=https://example.com
```

Go module 代理 proxy.golang.org；`/go` 路径分流到 dl.google.com 的 Go 工具链下载。

### OpenClaw

```shell
curl -fsSL https://example.com/openclaw.ai/install.sh | bash
```

install.sh / install-cli.sh 由 `/openclaw.ai` 源代理，脚本内的绝对地址（openclaw.ai、github.com、raw.githubusercontent.com、registry.npmjs.org、deb/rpm.nodesource.com、nodejs.org）会改写为镜像路径。限制：

- NodeSource 安装脚本执行后会把 `deb.nodesource.com` 写入 apt/yum 软件源，后续 Node.js 包由系统包管理器直连该域名，完全离网的机器需要自行处理这一步。
- 文档链接（docs.openclaw.ai）不改写，仅影响脚本文档跳转。
- 与任何安装脚本一样，执行前请先审阅脚本内容；本镜像只做透传与地址改写。

### 其他

- k8s.gcr.io / registry.k8s.io / ghcr.io 等注册表：路径模式 `https://example.com/ghcr.io/v2/...` 或子域名模式。
- Fedora EPEL/COPR、Kali、Chrome、Docker CE 安装包等见 Web 控制台“镜像源”卡片。

## 计划支持

- [x] Ubuntu/Debian/Kali/Alpine（含 Ubuntu security 更新源）
- [x] Python - PyPI / PyTorch
- [x] Docker - Dockerhub
- [x] Github
- [x] Node - Npm
- [x] Golang - Goproxy
- [x] Golang/Docker - download
- [x] Rust - Crates.io (sparse) / Rustup
- [x] Homebrew（API / bottles / brew、core git）
- [x] OpenClaw 安装脚本
- [x] 指定域名代理 githubusercontent、gist 等
- [ ] ......

其他源理论上也支持，目前仅作为 demo 使用，不做过多添加，需要添加的可以自行 fork 或 PR。

## 本地构建与开发

- **前端**：位于 `frontend/`，Vue 3 + Vite + TypeScript。开发时可在 `frontend` 目录执行 `npm run dev`；生产构建输出到 `frontend/dist`，由 Go 在编译时通过 `go:embed` 嵌入。
- **后端**：需先构建前端再编译 Go，否则 embed 目录不存在会报错。
  ```shell
  make build
  # 或分步：make frontend-build && go build -v -trimpath -o the-only-mirror ./
  ```
- **Docker 镜像**：Dockerfile 为多阶段构建，先构建前端再编译 Go，最终镜像仅包含二进制与 `config.json`。
- **测试**：`go test ./...`；`test/connectivity_test.go` 为真实链路冒烟测试，默认跳过，可用 `MIRROR_BASE_URL=http://127.0.0.1:8080 go test ./test/ -v` 对运行中的实例执行。

## 配置文件

除下方字段外，还支持 `host_aliases`（子域名到上游的映射）和 `source_templates`（按模板批量生成 sources），详见仓库内 `config.json` 示例。

sources 字段：

- `path`：请求路径前缀（匹配用）；`prefix`：转发到上游时保留/拼接的前缀；`strip_path`：为 `true` 时把匹配到的 `path` 段剥掉再拼 `prefix`（用于 `/ubuntu-security` → `/ubuntu` 这类路径改写）。
- `priority`：匹配顺序，数字大者先匹配；同优先级按 path 长度降序、再按 key 升序，保证确定性。注意：与 UA 兜底源（path 为空）共用同一 UA 的具体路径源，需要设置更高的 priority（如 `pytorch` 与 `pypi_web`）。
- `ua`：按 User-Agent 前缀匹配。
- `replaces`：响应改写。`type: "header"` 时改写指定响应头（多值头逐值改写）；默认改写响应正文，`src`/`dst` 支持 `<HOST>`、`<TLS_SCHEME>` 占位符（按请求实际 scheme 解析，兼容 Caddy 等反代的 `X-Forwarded-Proto`）。正文改写仅限 8 MiB 以内（gzip 解压限额 64 MiB），超限自动透传；改写成功后会清除失效的 `ETag`/`Digest`/`Content-MD5`。
- `rewrite_content_types`：额外允许改写正文的 Content-Type 列表（如 `application/octet-stream`、`binary/octet-stream`），默认仅 text/json/xml/javascript/shell 脚本类。

```json
{
  "port": 8080,
  "tls": false,
  "tls_redirect": false,
  "crt": "",
  "key": "",
  "sources": {
    "ubuntu": {
      "path": "/ubuntu",
      "mirror": "http://archive.ubuntu.com"
    },
    "ubuntu-security": {
      "path": "/ubuntu-security",
      "prefix": "/ubuntu",
      "strip_path": true,
      "mirror": "http://security.ubuntu.com"
    },
    "docker_auth": {
      "priority": 2,
      "ua": "docker",
      "path": "/token",
      "mirror": "https://auth.docker.io"
    },
    "docker_registry": {
      "priority": 1,
      "ua": "docker",
      "replaces": [
        { "type": "header", "header": "www-authenticate", "src": "https://", "dst": "<TLS_SCHEME>" },
        { "type": "header", "header": "www-authenticate", "src": "auth.docker.io", "dst": "<HOST>" }
      ],
      "mirror": "https://registry-1.docker.io"
    },
    "pypi_web": {
      "prefix": "/simple",
      "mirror": "https://pypi.org"
    }
  },
  "proxy": [
    "https://github.com"
  ]
}
```

## 其他项目

本项目的另一种实现：https://github.com/Jlan45/TheOnlyMirror
