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

## 计划支持

- [x] Ubuntu/Debian/CentOS/Kali/Alpine
- [x] Python - Pypi
- [x] Docker - Dockerhub
- [x] Github
- [x] Node - Npm
- [x] Golang - Goproxy
- [x] Golang/Docker - download
- [x] 指定域名代理 githubusercontent、gist 等
- [ ] ......

其他源理论上也支持，目前仅作为 demo 使用，不做过多添加，需要添加的可以自行 fork 或 PR。

## 使用

```shell
# Docker 换源
tee /etc/docker/daemon.json <<-'EOF'
{
  "registry-mirrors": ["https://example.com"]
}
EOF

# Linux 换源，仅作参考
sed -i "s/http.kali.org/example.com/g" /etc/apt/sources.list

# Python 换源
pip config set global.index-url https://example.com
pip install -i https://example.com -r requirements.txt

# Golang 换源
export GO111MODULE=on
export GOPROXY=https://example.com

# Node 换源
npm config set registry https://example.com

# 代理下载 GitHub 文件
curl -O https://example.com/github.com/huangzheng2016/TheOnlyMirror/archive/master.zip
wget https://example.com/raw.githubusercontent.com/huangzheng2016/TheOnlyMirror/main/README.md
```

上述示例中的 `https://example.com` 请替换为你的镜像站地址；若通过 Web 控制台访问，页面上的命令会自动使用当前域名。

## 本地构建与开发

- **前端**：位于 `frontend/`，Vue 3 + Vite + TypeScript。开发时可在 `frontend` 目录执行 `npm run dev`；生产构建输出到 `frontend/dist`，由 Go 在编译时通过 `go:embed` 嵌入。
- **后端**：需先构建前端再编译 Go，否则 embed 目录不存在会报错。
  ```shell
  make build
  # 或分步：make frontend-build && go build -v -trimpath -o the-only-mirror ./
  ```
- **Docker 镜像**：Dockerfile 为多阶段构建，先构建前端再编译 Go，最终镜像仅包含二进制与 `config.json`。

## 配置文件

除下方字段外，还支持 `host_aliases`（子域名到上游的映射）和 `source_templates`（按模板批量生成 sources），详见仓库内 `config.json` 示例。

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
      "mirror": "https://archive.ubuntu.com"
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
