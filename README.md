# TheOnlyMirror

既然镜像源太占空间，那为什么不能用反代源呢！

但加速代理域名那么多，为什么不尝试用同一个域名，代理所有的镜像源呢？

原理为不同应用请求镜像源时的UA/PATH/HOST/HEADER会有相应的特征，从而确定请求的镜像源。

如果采用Docker+Nginx反代方式部署，请务必打开HTTP/2支持

## 快速部署

感谢@jimyag的PR
```shell
docker run -d --restart=unless-stopped -p 8080:8080 ghcr.io/huangzheng2016/the-only-mirror
```

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

其他源理论上也支持，目前仅作为demo使用，不做过多添加，需要添加的可以自行fork或者pr

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
$ export GO111MODULE=on
$ export GOPROXY=https://example.com

# Node 换源
npm config set registry https://example.com

# 代理下载 github 文件 （TODO：302跳转未处理）
curl -O https://example.com/github.com/huangzheng2016/TheOnlyMirror/archive/master.zip
wget https://example.com/raw.githubusercontent.com/huangzheng2016/TheOnlyMirror/main/README.md
```

## 配置文件

```json
{
  "port":8080,//代理端口
  "tls":false,//是否使用TLS
  "tls_redirect":false,//在不使用TLS的情况下，打开此功能可以解决一些不支持TLS的问题
  "crt":"",//TLS证书
  "key":"",
  "sources":{
    "ubuntu":{
      "path":"/ubuntu",//镜像源路径
      "mirror":"https://archive.ubuntu.com"//镜像源地址
    },
    ......
    "docker_auth":{
      "priority":2,//优先级，优先级越大越先匹配，默认为0
      "ua":"docker",//匹配的User-Agent
      "path":"/token",//匹配的路径
      //当ua/path两者都存在时，需要同时满足
      "mirror":"https://auth.docker.io"
    },
    "docker_registry":{
      "priority":1,
      "ua":"docker",
      "replaces":[
        {
          "type":"header",//替换规则类型，header为替换请求头
          "header": "www-authenticate",//匹配头
          "src":"https://",//匹配目标
          "dst":"<TLS_SCHEME>"//如果tls_redirect为true，则会根据tls开关，替换为https或者http
        },
        {
          "type":"header",
          "header": "www-authenticate",
          "src":"auth.docker.io",
          "dst":"<HOST>"//替换为请求的Host
        }
      ],
      "mirror":"https://registry-1.docker.io"
    },
    ......
    "pypi_web":{
      ......
      "prefix":"/simple",//代理时需要加入的前缀
      ......
    },
    ......
  },
  "proxy":[
    "https://github.com",//允许代理下载的域名
    ......
  ]
}
```


## 其他项目

本项目的另一种实现：https://github.com/Jlan45/TheOnlyMirror
