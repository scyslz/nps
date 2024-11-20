# NPS
![](https://img.shields.io/github/stars/djylb/nps.svg)   ![](https://img.shields.io/github/forks/djylb/nps.svg)
![Release](https://github.com/djylb/nps/workflows/Release/badge.svg)
![GitHub All Releases](https://img.shields.io/github/downloads/djylb/nps/total)

[README](https://github.com/djylb/nps/blob/master/README.md)|[中文文档](https://github.com/djylb/nps/blob/master/README_zh.md)

nps是一款轻量级、高性能、功能强大的**内网穿透**代理服务器。目前支持**tcp、udp流量转发**，可支持任何**tcp、udp**上层协议（访问内网网站、本地支付接口调试、ssh访问、远程桌面，内网dns解析等等……），此外还**支持内网http代理、内网socks5代理**、**p2p等**，并带有功能强大的web管理端。

# 说明
由于[nps](https://github.com/ehang-io/nps)已经有二年多的时间没有更新了，存留了不少bug和未完善的功能。

此版本基于 nps 0.26 的基础上二次开发而来。

[Telegram](https://t.me/npsdev)

***DockerHub***： [NPS](https://hub.docker.com/r/duan2001/nps) [NPC](https://hub.docker.com/r/duan2001/npc)

## 简单安装说明 (具体还是参考 [官方文档](https://ehang-io.github.io/nps/) ，虽然已经过时了但也能凑合用)

注：如果是从其他分支切换到该分支的话建议按照下方说明重新执行安装命令 [#4](https://github.com/djylb/nps/issues/4) 。

### NPS
下载后解压到文件夹（注意Windows不要删除该文件夹）
```
# Linux (配置安装路径：/etc/nps/) (二进制安装路径：/usr/bin/)
./nps install
nps start|stop|restart|uninstall
# 更新
nps stop
nps-update update
nps start

# Windows (配置路径 C:\Program Files\nps\) (二进制路径：当前文件夹)
.\nps.exe install
.\nps.exe start|stop|restart|uninstall
# 更新
.\nps.exe stop
.\nps-update.exe update
.\nps.exe start
```

### NPC
下载后解压到文件夹（注意Windows不要删除该文件夹）
```
# Linux (二进制安装路径：/usr/bin/)
./npc install
/usr/bin/npc install -server=xxx:123 -vkey=xxx -type=tcp -tls_enable=true -log=off
npc start|stop|restart|uninstall
# 更新
npc stop
/usr/bin/npc-update update
npc start
# 查看参数说明
npc -h

# Windows (二进制路径：当前文件夹)
.\npc.exe install -server="xxx:123" -vkey="xxx" -type="tcp" -tls_enable="true" -log="off"
.\npc.exe start|stop|restart|uninstall
# 更新
.\npc.exe stop
.\npc-update.exe update
.\npc.exe start
# 查看参数说明
.\npc.exe -h
```
- 手动安装多开指南 （需要手动停止所有运行的服务才能正常更新，最好直接用Docker多开）[#9](https://github.com/djylb/nps/issues/9)

Windows （看懂下面命令再操作 [微软SC命令指南](https://learn.microsoft.com/zh-cn/windows-server/administration/windows-commands/sc-create)）
```
cmd /c 'sc create Npc1 binPath= "D:\tools\npc.exe -server=xxx:123 -vkey=xxx -type=tcp -tls_enable=true -log=off -debug=false" DisplayName= "nps内网穿透客户端1" start= auto'
```

Linux (根据下面示例编写systemd配置) (/etc/systemd/system/服务名称.service)
```
[Unit]
Description=一款轻量级、功能强大的内网穿透代理服务器。支持tcp、udp流量转发，支持内网http代理、内网socks5代理，同时支持snappy压缩、站点保护、加密传输、多路复用、header修改等。支持web图形化管理，集成多用户模式。
ConditionFileIsExecutable=/usr/bin/npc
 
Requires=network.target  
After=network-online.target syslog.target 
[Service]
LimitNOFILE=65536
StartLimitInterval=5
StartLimitBurst=10
ExecStart=/usr/bin/npc "-server=xxx:123" "-vkey=xxx "-type=tcp" "-debug=false" "-log=off"
Restart=always
RestartSec=120
[Install]
WantedBy=multi-user.target
```

### Docker
***DockerHub***： [NPS](https://hub.docker.com/r/duan2001/nps) [NPC](https://hub.docker.com/r/duan2001/npc)
```
# NPS
docker pull duan2001/nps
docker run -d --restart=always --name nps --net=host -v <本机conf目录>:/conf duan2001/nps

# NPC
docker pull duan2001/npc
docker run -d --restart=always --name npc --net=host duan2001/npc -server=xxxx:123 -vkey=xxxx,xxxx -tls_enable=true -log=off
```

## 补充说明
- 域名转发的HTTPS证书和密钥位置支持填写路径或证书文本内容
  
  其中路径支持绝对路径和相对路径，不过最好填写绝对路径，相对路径是以nps二进制文件所在路径为基准。
  
  此外docker映射的文件夹内文件不支持软链接，有需要请使用硬链接。
- 客户端命令行方式启动支持多个隧道ID，使用逗号拼接，示例：`npc -server=xxx:8024 -vkey=ytkpyr0er676m0r7,iwnbjfbvygvzyzzt`
- 当需要在NPS前添加反向代理时可以通过插入头（X-NPS-Http-Only: password）来避免301重定向和插入真实IP
- 域名转发的模式指的是访问NPS的模式而不是后端服务器模式，正常情况下目标应该填写后端HTTP端口，另外不要使用Proxy Protocol(Websocket兼容存在问题)，通过 X-Forwarded-For 或 X-Real-IP 获取真实IP

  如果后端只有HTTPS的话可以通过将模式配置为HTTPS，同时NPS证书位置留空则即可，注意后端证书要配置正确，如果后端支持可以通过Proxy Protocol获取真实IP
- NPS日志配置 nps.conf
```
# 日志级别 (0-7) LevelEmergency->0  LevelAlert->1 LevelCritical->2 LevelError->3 LevelWarning->4 LevelNotice->5 LevelInformational->6 LevelDebug->7
log_level=6
# 日志路径，留空则使用默认路径(路径|off|docker)
# 填路径输出到路径 填off关闭日志文件输出 填docker输出到docker控制台日志
log_path=off
# 是否按日期保存日志(true|false)
log_daily=false
# 允许存在的日志总文件个数
log_max_files=10
# 允许保存日志的最大天数
log_max_days=7
# 单个日志文件的最大大小MB，超过大小或日志超过100000行则新增日志文件
log_max_size=2
```
  NPC使用 ```npc -h``` 查看用法

## 更新日志
### DEV
- 2024-11-20 v0.26.29
  - 待定，优先修BUG，新功能随缘更新

### Stable
- 2024-11-20 v0.26.28
  - 修复NPC在docker环境下使用配置文件启动失败问题
  - 应用户要求使用旧版Web页面风格
  - 完善配置文件说明

<details>

- 2024-11-19 v0.26.27
  - 完善界面翻译和提示内容
  - 修复https just proxy
  - 域名转发也支持Proxy Protocol

     (仅用于代理后端HTTPS时传递真实IP，正常情况下请直接使用 X-Forwarded-For 或 X-Real-IP 获取真实IP)

- 2024-11-16 v0.26.26
  - 增强服务端日志控制
  - 修复停止后已存在的TCP通道不会立即关闭
  - 添加Proxy Protocol支持

- 2024-11-14  v0.26.25
  - 调整界面显示
  - 增强日志控制 (具体见NPC命令行参数，支持开关、自动删除等功能)
  - 添加旧版本编译（支援win7，请下载old结尾的压缩包）

- 2024-11-09  v0.26.24
  - 修复语言翻译缺失
  - 请求静态文件携带版本号，避免浏览器缓存旧文件（升级后记得替换web目录）
  - 优化代码逻辑和效率
  - 修复通配符匹配优先级（优先完全匹配Host，通配符根据匹配程度确定优先级）
  - 修复根据路径分流功能

- 2024-11-08  v0.26.23  
  - 合并同类项目分支补丁更新
    - 客户端增加创建时间 [yisier](https://github.com/yisier/nps)
    - 增加从下列选择客户端、排序 [dreamskr](https://github.com/dreamskr/nps)

- 2024-10-28  v0.26.22  
  - 修复多目标负载均衡不生效的问题
    （注意最后一行不要输回车）

- 2024-10-28  v0.26.21  
  - 修复websocket支持(支持类似homeassistant的网站反向代理)
    删除websocket的认证操作，交给应用层进行处理
  - 重构优化代码（目前简单测试功能正常，CPU占用也不高，不知道引入没引入新BUG，代码维护的人多了有点乱腾）
  - 新增X-NPS-Http-Only头支持，当需要在NPS前添加反向代理时可以通过插入头（X-NPS-Http-Only: password）
    此时可以反向代理http_proxy_port避免301重定向和添加真实IP

- 2024-10-25  v0.26.20  
  - 修复ipv6支持
  - 同时支持传入证书路径和证书文本内容
  - http、socket5同时使用全局用户和mutli user认证
  - 修复绕过认证漏洞
  - 美化UI界面
  - 合并上游所有分叉的安全补丁和更新（总之修了一堆BUG）
  - 更新相关依赖

- 2024-06-01  v0.26.19  
  - golang 版本升级到 1.22.
  - 增加自动https，自动将http 重定向（301）到 https.  
  - 客户端命令行方式启动支持多个隧道ID，使用逗号拼接，示例：`npc -server=xxx:8024 -vkey=ytkpyr0er676m0r7,iwnbjfbvygvzyzzt` .
  - 移除 nps.conf 参数 `https_just_proxy` , 调整 https 处理逻辑，如果上传了 https 证书，则由nps负责SSL (此方式可以获取真实IP)，
      否则走端口转发模式（使用本地证书,nps 获取不到真实IP）， 如下图所示。    
    ![image](image/new/https.png)



- 2024-02-27  v0.26.18  
  ***新增***：nps.conf 新增 `tls_bridge_port=8025` 参数，当 `tls_enable=true` 时，nps 会监听8025端口，作为 tls 的连接端口。  
             客户端可以选择连接 tls 端口或者非 tls 端口： `npc.exe  -server=xxx:8024 -vkey=xxx` 或 `npc.exe  -server=xxx:8025 -vkey=xxx -tls_enable=true`
  
  
- 2024-01-31  v0.26.17  
  ***说明***：考虑到 npc 历史版本客户端众多，版本号不同旧版本客户端无法连接，为了兼容，仓库版本号将继续沿用 0.26.xx


- 2024-01-02  v0.27.01  (已作废，功能移动到v0.26.17 版本)  
  ***新增***：tls 流量加密，(客户端忽略证书校验，谨慎使用，客户端与服务端需要同时开启，或同时关闭)，使用方式：   
             服务端：nps.conf `tls_enable=true`;    
             客户端：npc.conf `tls_enable=true` 或者 `npc.exe  -server=xxx -vkey=xxx -tls_enable=true`  

  
- 2023-06-01  v0.26.16  
  ***修复***：https 流量不统计 Bug 修复。  
  ***新增***：新增全局黑名单IP，用于防止被肉鸡扫描端口或被恶意攻击。  
  ***新增***：新增客户端上次在线时间。


- 2023-02-24  v0.26.15  
  ***修复***：更新程序 url 更改到当前仓库中   
  ***修复***：nps 在外部路径启动时找不到配置文件  
  ***新增***：增加 nps 启动参数，`-conf_path=D:\test\nps`,可用于加载指定nps配置文件和web文件目录。  
  ***window 使用示例：***  
  直接启动：`nps.exe -conf_path=D:\test\nps`  
  安装：`nps.exe install -conf_path=D:\test\nps`    
  安装启动：`nps.exe start`      

  ***linux 使用示例：***    
  直接启动：`./nps -conf_path=/app/nps`  
  安装：`./nps install -conf_path=/app/nps`  
  安装启动：`nps start -conf_path=/app/nps`  



- 2022-12-30  v0.26.14  
  ***修复***：API 鉴权漏洞修复


- 2022-12-19  
***修复***：某些场景下丢包导致服务端意外退出  
***优化***：新增隧道时，不指定服务端口时，将自动生成端口号  
***优化***：API返回ID, `/client/add/, /index/addhost/，/index/add/ `   
***优化***：域名解析、隧道页面，增加[唯一验证密钥]，方便搜查  


- 2022-10-30   
***新增***：在管理面板中新增客户端时，可以配置多个黑名单IP，用于防止被肉鸡扫描端口或被恶意攻击。  
***优化***：0.26.12 版本还原了注册系统功能，使用方式和以前一样。无论是否注册了系统服务，直接执行 nps 时只会读取当前目录下的配置文件。


- 2022-10-27  
***新增***：在管理面板登录时开启验证码校验，开启方式：nps.conf `open_captcha=true`，感谢 [@dongFangTuring](https://github.com/dongFangTuring) 提供的PR  

  
- 2022-10-24:     
***修复***：HTTP协议支持WebSocket(稳定性待测试)
  

- 2022-10-21:   
***修复***：HTTP协议下实时统计流量，能够精准的限制住流量（上下行对等）  
***优化***：删除HTTP隧道时，客户端已用流量不再清空


- 2022-10-19:  
***BUG***：在TCP协议下，流量统计有问题，只有当连接断开时才会统计流量。例如，限制客户端流量20m,当传输100m的文件时，也能传输成功。  
***修复***：TCP协议下实时统计流量，能够精准的限制住流量（上下行对等）  
***优化***：删除TCP隧道时，客户端已用流量不再清空
![image](image/new/tcp_limit.png)


- 2022-09-14:  
修改NPS工作目录为当前可执行文件目录（即配置文件和nps可执行文件放在同一目录下，直接执行nps文件即可），去除注册系统服务，启动、停止、升级等命令

</details>
