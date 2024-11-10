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

## 更新日志
- 2024-11-09  v0.26.24
  - 修复语言翻译缺失
  - 请求静态文件携带版本号，避免浏览器缓存旧文件（升级后记得替换web目录）
  - 优化代码逻辑和效率
  - 修复通配符匹配优先级（优先完全匹配Host，通配符根据匹配程度确定优先级）
  - 修复根据路径分流功能

<details>

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
