# NPS 内网穿透

[![GitHub stars](https://img.shields.io/github/stars/djylb/nps.svg)](https://github.com/djylb/nps)
[![GitHub forks](https://img.shields.io/github/forks/djylb/nps.svg)](https://github.com/djylb/nps)
[![Release](https://github.com/djylb/nps/workflows/Release/badge.svg)](https://github.com/djylb/nps/actions)
[![GitHub All Releases](https://img.shields.io/github/downloads/djylb/nps/total)](https://github.com/djylb/nps/releases)

- [README](https://github.com/djylb/nps/blob/master/README.md) | [中文文档](https://github.com/djylb/nps/blob/master/README_zh.md)

---

## 简介

NPS 是一款轻量高效的内网穿透代理服务器，支持多种协议（TCP、UDP、HTTP、SOCKS5 等）转发。它提供直观的 Web 管理界面，使得内网资源能安全、便捷地在外网访问，同时满足多种复杂场景的需求。

由于[nps](https://github.com/ehang-io/nps)停更已久，本仓库基于 nps 0.26 整合社区更新二次开发而来。

---

## 主要特性

- **多协议支持**  
  TCP/UDP 转发、HTTP/SOCKS5 代理、P2P 模式等，满足各种内网访问场景。

- **跨平台部署**  
  支持 Linux、Windows 等主流平台，可轻松安装为系统服务。

- **Web 管理界面**  
  实时监控流量、连接情况以及客户端状态，操作简单直观。

- **安全与扩展**  
  内置加密传输、流量限制、证书管理等多重功能，保障数据安全。

---

## 安装与使用
### Docker 部署

***DockerHub***： [NPS](https://hub.docker.com/r/duan2001/nps) [NPC](https://hub.docker.com/r/duan2001/npc)

***GHCR***： [NPS](https://github.com/djylb/nps/pkgs/container/nps) [NPC](https://github.com/djylb/nps/pkgs/container/npc)


#### NPS 服务端
```bash
docker pull duan2001/nps
docker run -d --restart=always --name nps --net=host -v <本机配置目录>:/conf -v /etc/localtime:/etc/localtime:ro duan2001/nps
```

#### NPC 客户端
```bash
docker pull duan2001/npc
docker run -d --restart=always --name npc --net=host duan2001/npc -server=xxx:123 -vkey=key1,key2 -tls_enable=true -log=off
```

### 服务端安装

#### Linux
```bash
# 安装（默认配置路径：/etc/nps/；二进制文件路径：/usr/bin/）
./nps install
nps start|stop|restart|uninstall

# 更新
nps stop
nps-update update
nps start
```

#### Windows
```powershell
.\nps.exe install
.\nps.exe start|stop|restart|uninstall

# 更新
.\nps.exe stop
.\nps-update.exe update
.\nps.exe start
```

### 客户端安装

#### Linux
```bash
./npc install
/usr/bin/npc install -server=xxx:123 -vkey=xxx -type=tcp -tls_enable=true -log=off
npc start|stop|restart|uninstall

# 更新
npc stop
/usr/bin/npc-update update
npc start
```

#### Windows
```powershell
.\npc.exe install -server="xxx:123" -vkey="xxx" -type="tcp" -tls_enable="true" -log="off"
.\npc.exe start|stop|restart|uninstall

# 更新
.\npc.exe stop
.\npc-update.exe update
.\npc.exe start
```

> **提示：** 客户端支持同时传入多个隧道 ID，示例：  
> `npc -server=xxx:8024 -vkey=key1,key2`

---

## 其他说明

- **IPv6 支持**  
  默认启用，无需额外配置。

- **HTTPS 与反向代理**  
  支持配置证书（绝对路径或证书内容），并可在反向代理场景中通过添加特定 HTTP 头（如 `X-NPS-Http-Only`）跳过重定向。

- **日志管理**  
  可通过配置文件灵活设置日志级别、保存路径、文件大小与保留数量。

更多详细配置请参考 [文档](https://d-jy.net/docs/nps/)（部分内容可能已更新）。

---

## 更新日志

- **v0.26.38 (2025-03-14)**  
  域名转发支持 HTTP/2，优化证书缓存及依赖更新。

- **v0.26.37 (2025-03-13)**  
  新增后端 HTTPS 支持，改进 CORS 自动补全与 SNI 识别。

更多历史更新记录请参阅项目 [Releases](https://github.com/djylb/nps/releases)。

---

## 贡献与反馈

- **提问前请先查阅：**  
  [文档](https://d-jy.net/docs/nps//) 与 [issues](https://github.com/djylb/nps/issues)

- **欢迎参与：**  
  提交 PR、反馈问题或建议，共同推动项目发展。

- **讨论交流：**  
  加入 [Telegram 交流群](https://t.me/npsdev) 与其他用户交流经验。

