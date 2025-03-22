# NPS Intranet Tunneling

[![GitHub stars](https://img.shields.io/github/stars/djylb/nps.svg)](https://github.com/djylb/nps)
[![GitHub forks](https://img.shields.io/github/forks/djylb/nps.svg)](https://github.com/djylb/nps)
[![Release](https://github.com/djylb/nps/workflows/Release/badge.svg)](https://github.com/djylb/nps/actions)
[![GitHub All Releases](https://img.shields.io/github/downloads/djylb/nps/total)](https://github.com/djylb/nps/releases)

- [中文文档](https://github.com/djylb/nps/blob/master/README.md)

---

## Introduction

NPS is a lightweight and efficient intranet tunneling proxy server that supports forwarding multiple protocols (TCP, UDP, HTTP, SOCKS5, etc.). It features an intuitive web management interface that allows secure and convenient access to intranet resources from external networks, addressing a wide range of complex scenarios.

Due to the long-term discontinuation of [NPS](https://github.com/ehang-io/nps), this repository is a community-driven, updated fork based on nps 0.26.

- **Before asking questions, please check:** [Documentation](https://d-jy.net/docs/nps/) and [Issues](https://github.com/djylb/nps/issues)
- **Contributions welcome:** Submit PRs, provide feedback or suggestions, and help drive the project forward.
- **Join the discussion:** Connect with other users in our [Telegram Group](https://t.me/npsdev).

---

## Key Features

- **Multi-Protocol Support**  
  Offers TCP/UDP forwarding, HTTP/SOCKS5 proxy, P2P mode, and more to suit various intranet access scenarios.

- **Cross-Platform Deployment**  
  Compatible with major platforms like Linux and Windows, with easy installation as a system service.

- **Web Management Interface**  
  Provides real-time monitoring of traffic, connection statuses, and client performance in an intuitive interface.

- **Security and Extensibility**  
  Includes built-in encryption, traffic limiting, certificate management, and other features to ensure data security.

---

## Installation and Usage

For more detailed configuration options, please refer to the [Documentation](https://d-jy.net/docs/nps/) (some sections may be outdated).

### Docker Deployment

**DockerHub:**  [NPS](https://hub.docker.com/r/duan2001/nps) | [NPC](https://hub.docker.com/r/duan2001/npc)

**GHCR:**  [NPS](https://github.com/djylb/nps/pkgs/container/nps) | [NPC](https://github.com/djylb/nps/pkgs/container/npc)

#### NPS Server
```bash
docker pull duan2001/nps
docker run -d --restart=always --name nps --net=host -v $(pwd)/conf:/conf -v /etc/localtime:/etc/localtime:ro duan2001/nps
```

#### NPC Client
```bash
docker pull duan2001/npc
docker run -d --restart=always --name npc --net=host duan2001/npc -server=xxx:123 -vkey=key1,key2 -tls_enable=true -log=off
```

### Server Installation

#### Linux
```bash
# Install (default configuration path: /etc/nps/; binary file path: /usr/bin/)
./nps install
nps start|stop|restart|uninstall

# Update
nps stop
nps-update update
nps start
```

#### Windows
```powershell
.\nps.exe install
.\nps.exe start|stop|restart|uninstall

# Update
.\nps.exe stop
.\nps-update.exe update
.\nps.exe start
```

### Client Installation

#### Linux
```bash
./npc install
/usr/bin/npc install -server=xxx:123 -vkey=xxx -type=tcp -tls_enable=true -log=off
npc start|stop|restart|uninstall

# Update
npc stop
/usr/bin/npc-update update
npc start
```

#### Windows
```powershell
.\npc.exe install -server="xxx:123" -vkey="xxx" -type="tcp" -tls_enable="true" -log="off"
.\npc.exe start|stop|restart|uninstall

# Update
.\npc.exe stop
.\npc-update.exe update
.\npc.exe start
```

> **Note:** The client supports passing multiple tunnel IDs simultaneously, e.g.:  
> `npc -server=xxx:8024 -vkey=key1,key2`
