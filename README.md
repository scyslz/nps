# NPS

![](https://img.shields.io/github/stars/djylb/nps.svg)   ![](https://img.shields.io/github/forks/djylb/nps.svg)
![Release](https://github.com/djylb/nps/workflows/Release/badge.svg)
![GitHub All Releases](https://img.shields.io/github/downloads/djylb/nps/total)

[README](https://github.com/djylb/nps/blob/master/README.md)|[中文文档](https://github.com/djylb/nps/blob/master/README_zh.md)

nps是一款轻量级、高性能、功能强大的**内网穿透**代理服务器。目前支持**tcp、udp流量转发**，可支持任何**tcp、udp**上层协议（访问内网网站、本地支付接口调试、ssh访问、远程桌面，内网dns解析等等……），此外还**支持内网http代理、内网socks5代理**、**p2p等**，并带有功能强大的web管理端。

# 说明

由于[nps](https://github.com/ehang-io/nps)已经有二年多的时间没有更新了，存留了不少bug和未完善的功能。

此版本基于 nps 0.26 的基础上二次开发而来。

[Telegram](https://t.me/npsdev) 交流群

提问说明：看完本页说明和官方文档后再提问，优先在 [issues](https://github.com/djylb/nps/issues) 提问，不要提重复问题。仅对本仓库最新版本提供支持，提问前请先检查NPS版本号是否为最新版。

问题描述越详细获得支持的可能性越大，其他讨论可以使用上方群。

***DockerHub***： [NPS](https://hub.docker.com/r/duan2001/nps) [NPC](https://hub.docker.com/r/duan2001/npc)

## 简单安装说明

(具体还是参考 [官方文档](https://ehang-io.github.io/nps/) ，虽然已经过时了但也能凑合用)

注：如果是从其他分支切换到该分支的话建议按照下方说明重新执行安装命令 [#4](https://github.com/djylb/nps/issues/4) 。

***旧版Windows请使用old结尾的压缩包，同时不支持命令更新，如需升级请手动替换文件。***

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

开启自启：```systemctl enable Npc``` 启动服务：```systemctl start Npc```

注：需要会使用systemctl指令，不会请自行学习 [Systemd](https://docs.redhat.com/zh-cn/documentation/red_hat_enterprise_linux/9/html/configuring_basic_system_settings/managing-system-services-with-systemctl_managing-systemd#starting-a-system-service_managing-system-services-with-systemctl) 。

### Docker

***DockerHub***： [NPS](https://hub.docker.com/r/duan2001/nps) [NPC](https://hub.docker.com/r/duan2001/npc)

```
# NPS
# 根据下面链接创建 nps.conf 配置文件
# https://github.com/djylb/nps/blob/master/conf/nps.conf
docker pull duan2001/nps
docker run -d --restart=always --name nps --net=host -v <本机conf目录>:/conf -v /etc/localtime:/etc/localtime:ro duan2001/nps

# NPC
docker pull duan2001/npc
docker run -d --restart=always --name npc --net=host duan2001/npc -server=xxxx:123 -vkey=xxxx,xxxx -tls_enable=true -log=off
```

## 补充说明

- 关于ipv6问题，不需要修改任何配置，默认配置就已经在双栈协议上监听了
- 域名转发的HTTPS证书和密钥位置支持填写路径或证书文本内容

  其中路径支持绝对路径和相对路径，不过最好填写绝对路径，相对路径是以nps二进制文件所在路径为基准。

  此外docker映射的文件夹内文件不支持软链接，有需要请使用硬链接。
- 客户端命令行方式启动支持多个隧道ID，使用逗号拼接，示例：`npc -server=xxx:8024 -vkey=ytkpyr0er676m0r7,iwnbjfbvygvzyzzt`
- 当需要在NPS前添加反向代理时可以通过插入头（X-NPS-Http-Only: password）来避免301重定向和插入真实IP。
- Auto CORS功能是自动插入允许跨域头部，允许跨域访问，不过最好还是在后端实现。
- 域名转发的模式指的是访问NPS的模式而不是后端服务器模式，正常情况下目标应该填写后端HTTP端口，通过 X-Forwarded-For 或 X-Real-IP 获取真实IP

  如果后端只有HTTPS的话，只需将后端类型选为HTTPS即可，如果不想在NPS配置证书只需要开启后端处理HTTPS (仅转发)即可，注意后端证书要配置正确。

  注意域名转发中的Proxy Protocol功能只在仅转发HTTPS情况下生效

  由后端处理HTTPS (仅转发)关闭时的处理优先级：用户自定义证书 > 默认证书 > 由后端处理HTTPS (仅转发)

- 到期时间限制，在nps.conf里配置allow_time_limit=true后即可在网页使用。注意服务器时区，格式随便填即可，自动识别（支持时间戳），留空关闭。例如：2025-01-01（指定东八时区：2025-01-01 00:00:00 +0800 CST）
- NPS日志配置 nps.conf

```
# 日志级别 (0-7) LevelEmergency->0  LevelAlert->1 LevelCritical->2 LevelError->3 LevelWarning->4 LevelNotice->5 LevelInformational->6 LevelDebug->7
log_level=7
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

- X-NPS-Http-Only功能NGINX配置示例

```
# 这里填NPS的http_proxy_port端口
proxy_pass http://127.0.0.1:80;
proxy_http_version 1.1;
proxy_set_header Upgrade $http_upgrade;
proxy_set_header Connection $http_connection;
proxy_set_header Host $http_host;
# 这里填NPS配置文件中填写的密码
proxy_set_header X-NPS-Http-Only "password";
proxy_set_header X-Real-IP $remote_addr;
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
proxy_redirect off;
proxy_buffering off;
```

## 更新日志

### DEV

- 2025-03-14 v0.26.39

  - 待定，优先修BUG，新功能随缘更新

### Stable
- 2025-03-14 v0.26.38
  - 域名转发支持HTTP/2
  - 当配置请求域名修时同时修改Origin头避免后端监测
  - 调整域名编辑页面逻辑
  - 更新相关依赖，修复CVE-2025-22870
  - 使用 [XTLS/go-win7](https://github.com/XTLS/go-win7) 编译旧版代码支持Win7
  - 整理仓库代码
  - 优化域名查找算法

- 2025-03-13 v0.26.37
  - 优化域名转发列表显示
  - 新增GHCR容器仓库
  - 重写整个域名转发模块
  - 优化域名匹配逻辑
  - 新增后端HTTPS支持
  - 新增自动补全CORS头部功能 (自动插入允许跨域头)
  - 新增流量连接数到期限制状态返回
  - 更新Go版本至1.24
  - 优化证书缓存逻辑
  - 优化SNI识别支持H2
  - 自动监测证书文件是否修改

- 2025-02-06 v0.26.36
  - 重写HTTPS模块，提高响应速度
  - 添加证书缓存功能
  - 修复潜在的连接数统计问题
  - 添加客户端到期日期限制 [#15](https://github.com/djylb/nps/issues/15)

    需要在nps.conf文件中添加下面配置才能使用

```
#时间限制
allow_time_limit=true
```

- 2025-1-30 v0.26.35
  - 优化密钥生成方式
  - 优化web页面显示逻辑
  - 优化域名转发速度 [#34](https://github.com/djylb/nps/issues/34)
  - 默认证书支持 [#35](https://github.com/djylb/nps/issues/35)

    注意：本次更新后原有的由后端处理HTTPS需在网页上重新配置把开关打开。

    证书留空则使用默认证书如果默认证书仍不存在则使用仅转发由后端处理SSL，同时为后端处理HTTPS添加开关选项。

<details>

- 2025-1-12 v0.26.34
  - 调整连接处理逻辑
  - 更新相关依赖库
  - 调整npc默认配置

- 2024-12-19 v0.26.33
  - 临时修复Socks5流量不计入客户端总流量问题 [#29](https://github.com/djylb/nps/issues/29)
  - 更新相关依赖库（安全更新）

- 2024-12-03 v0.26.32
  - 修复Proxy Protocol在开启客户端加密压缩后不能正常工作问题 [#26](https://github.com/djylb/nps/issues/26)
  - 修复TCP高并发下读写冲突问题
  - 调整域名转发下Proxy Protocol功能仅在后端处理HTTPS时生效，避免用户错误配置
  - 优化连接超时处理逻辑

- 2024-11-26 v0.26.31
  - 自动创建空json配置，避免启动失败

- 2024-11-22 v0.26.30
  - 开启allow_local_proxy后将自动生成虚拟客户端配置用于实现本机代理
  - Web编辑页面增加返回按钮
  - 在客户端编辑页面添加清空流量统计功能 [#5](https://github.com/djylb/nps/issues/5)
  - allow_local_proxy安全漏洞修复

- 2024-11-21 v0.26.29
  - 修复编辑界面证书显示和详情页面排版
  - 普通用户登录时隐藏服务器相关信息 [#7](https://github.com/djylb/nps/issues/7)
  - 隐藏NPS Web管理界面特征 (通过配置web_base_url可避免特征扫描)
  - 修复添加页面自动选中客户端

- 2024-11-20 v0.26.28
  - 修复NPC在docker环境下使用配置文件启动失败问题
  - 应用户要求使用旧版Web页面风格
  - 完善配置文件说明

- 2024-11-19 v0.26.27
  - 完善界面翻译和提示内容
  - 修复https just proxy
  - 域名转发也支持Proxy Protocol

    (仅用于代理后端HTTPS时传递真实IP，正常情况下请直接使用 X-Forwarded-For 或 X-Real-IP 获取真实IP)

- 2024-11-16 v0.26.26
  - 增强服务端日志控制
  - 修复停止后已存在的TCP通道不会立即关闭
  - 添加Proxy Protocol支持

- 2024-11-14 v0.26.25
  - 调整界面显示
  - 增强日志控制 (具体见NPC命令行参数，支持开关、自动删除等功能)
  - 添加旧版本编译（支援win7，请下载old结尾的压缩包）

- 2024-11-09 v0.26.24
  - 修复语言翻译缺失
  - 请求静态文件携带版本号，避免浏览器缓存旧文件（升级后记得替换web目录）
  - 优化代码逻辑和效率
  - 修复通配符匹配优先级（优先完全匹配Host，通配符根据匹配程度确定优先级）
  - 修复根据路径分流功能

- 2024-11-08 v0.26.23
  - 合并同类项目分支补丁更新
    - 客户端增加创建时间 [yisier](https://github.com/yisier/nps)
    - 增加从下列选择客户端、排序 [dreamskr](https://github.com/dreamskr/nps)

- 2024-10-28 v0.26.22
  - 修复多目标负载均衡不生效的问题
    （注意最后一行不要输回车）

- 2024-10-28 v0.26.21
  - 修复websocket支持(支持类似homeassistant的网站反向代理)
    删除websocket的认证操作，交给应用层进行处理
  - 重构优化代码（目前简单测试功能正常，CPU占用也不高，不知道引入没引入新BUG，代码维护的人多了有点乱腾）
  - 新增X-NPS-Http-Only头支持，当需要在NPS前添加反向代理时可以通过插入头（X-NPS-Http-Only: password）
    此时可以反向代理http_proxy_port避免301重定向和添加真实IP

- 2024-10-25 v0.26.20
  - 修复ipv6支持
  - 同时支持传入证书路径和证书文本内容
  - http、socket5同时使用全局用户和mutli user认证
  - 修复绕过认证漏洞
  - 美化UI界面
  - 合并上游所有分叉的安全补丁和更新（总之修了一堆BUG）
  - 更新相关依赖

</details>
