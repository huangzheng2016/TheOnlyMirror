export type ProfileId = 'docker' | 'linux' | 'python' | 'node' | 'go' | 'github'

export interface Profile {
  id: ProfileId
  label: string
  desc: string
  note: string
  buildCommands: (origin: string) => string
}

export const profiles: Profile[] = [
  {
    id: 'docker',
    label: 'Docker',
    desc: 'registry mirror',
    note: '写入 /etc/docker/daemon.json 并重启 Docker 后生效。',
    buildCommands: (origin) => `sudo tee /etc/docker/daemon.json <<-'EOF'
{
  "registry-mirrors": ["${origin}"]
}
EOF
sudo systemctl daemon-reload
sudo systemctl restart docker`,
  },
  {
    id: 'linux',
    label: 'Linux',
    desc: 'apt / apk 源替换',
    note: '按发行版选择对应命令，替换后执行 apt-get update 或 apk update。',
    buildCommands: (origin) => `# Ubuntu / Debian
sudo sed -i "s|http://archive.ubuntu.com/ubuntu|${origin}/ubuntu|g" /etc/apt/sources.list
sudo sed -i "s|https://deb.debian.org/debian|${origin}/debian|g" /etc/apt/sources.list

# Kali / Alpine
sudo sed -i "s|https://http.kali.org/kali|${origin}/kali|g" /etc/apt/sources.list
sudo sed -i "s|https://dl-cdn.alpinelinux.org/alpine|${origin}/alpine|g" /etc/apk/repositories`,
  },
  {
    id: 'python',
    label: 'Python',
    desc: 'pip / PyPI',
    note: '设置全局 index-url，或单次安装时用 -i 指定。',
    buildCommands: (origin) => `pip config set global.index-url ${origin}
# 或单次使用
pip install -i ${origin} -r requirements.txt`,
  },
  {
    id: 'node',
    label: 'Node',
    desc: 'npm registry',
    note: '对当前用户生效；CI 中可用 npm config set registry 或 .npmrc。',
    buildCommands: (origin) => `npm config set registry ${origin}

# 还原官方源
npm config set registry https://registry.npmjs.org`,
  },
  {
    id: 'go',
    label: 'Go',
    desc: 'GOPROXY',
    note: 'go env -w 写入全局环境，go mod download 即走代理。',
    buildCommands: (origin) => `go env -w GO111MODULE=on
go env -w GOPROXY=${origin}`,
  },
  {
    id: 'github',
    label: 'GitHub',
    desc: '文件下载加速',
    note: '按路径前缀代理 github.com 与 raw.githubusercontent.com。',
    buildCommands: (origin) => `curl -LO ${origin}/github.com/owner/repo/archive/refs/heads/main.zip
wget ${origin}/raw.githubusercontent.com/owner/repo/main/README.md`,
  },
]
