# **启动指南**

## 1. NPS 服务器

下载并解压 **NPS 服务器端** 压缩包，进入解压后的文件夹。

### **1.1 执行安装**
#### **Linux / macOS**
```bash
sudo ./nps install
```
#### **Windows**
以 **管理员身份** 运行 `cmd` 或 `PowerShell`，进入安装目录：
```powershell
nps.exe install
```

---

### **1.2 启动服务**
#### **Linux / macOS**
```bash
sudo nps start
```
#### **Windows**
```powershell
nps.exe start
```

📌 **安装后的二进制文件及配置目录**：
- **Windows**
  - 配置文件目录：`C:\Program Files\nps`
  - 二进制路径：安装目录（当前文件夹）
- **Linux / macOS**
  - 配置文件目录：`/etc/nps`
  - 二进制路径：`/usr/bin/nps`

📌 **停止/重启服务**
```bash
nps stop      # 停止服务
nps restart   # 重启服务
```

📌 **卸载 NPS**
```bash
nps uninstall
```

> **⚠️ Windows 用户请勿删除当前目录下的二进制文件！** `nps.exe` 必须保持在 **原始解压目录** 内，否则无法运行。

---

### **1.3 日志与调试**
📌 **如果发现未启动成功**
- **停止服务后手动运行调试**
  ```bash
  nps stop
  ./nps   # Linux/macOS 运行
  nps.exe  # Windows 运行
  ```
- **查看日志**  
  📌 **日志具体位置在 `nps.conf` 里配置**
  - **Windows**: 运行目录下的 `nps.log`
  - **Linux/macOS**: `/var/log/nps.log`

---

### **1.4 访问 Web 管理端**
- 打开浏览器，访问：
  ```
  http://<服务器IP>:8080
  ```
  （默认 Web 端口为 `8080`）
- 登录：
  ```
  用户名: admin
  密码: 123
  ```
  **⚠️ 正式使用请修改默认密码！**

- **创建客户端** 以便后续连接。

---

## 2. NPC 客户端

下载并解压 **NPC 客户端** 压缩包，进入解压目录。

---

### **2.1 获取启动命令**
- **进入 Web 管理端**
- **点击客户端前的 `+` 号**
- **复制启动命令**

---

### **2.2 直接运行（测试用）**
#### **Linux**
```bash
./npc -server=xxx:123 -vkey=xxx -type=tcp -tls_enable=true -log=off
```
#### **Windows**
```powershell
npc.exe -server="xxx:123" -vkey="xxx" -type="tcp" -tls_enable="true" -log="off"
```
> **⚠️ PowerShell 运行时，请用双引号括起命令参数！**

---

### **2.3 安装服务并启动**
#### **Linux**
```bash
./npc install -server=xxx:123 -vkey=xxx -type=tcp -tls_enable=true -log=off
./npc start
```
#### **Windows**
```powershell
npc.exe install -server="xxx:123" -vkey="xxx" -type="tcp" -tls_enable="true" -log="off"
npc.exe start
```
> **⚠️ PowerShell 运行时，请用双引号括起命令参数！**

📌 **安装后的二进制文件及配置目录**：
- **Windows**
  - 配置文件目录：`C:\Program Files\npc`
  - 二进制路径：安装目录（当前文件夹）
- **Linux**
  - 配置文件目录：`/etc/npc`
  - 二进制路径：`/usr/bin/npc`

> **⚠️ Windows 用户请勿删除当前目录下的二进制文件！** `npc.exe` 必须保持在 **原始解压目录** 内，否则无法运行。

📌 **卸载 NPC**
```bash
npc uninstall
```

---

### **2.4 手动注册为系统服务（多开适用）**
📌 **直接执行 `install` 命令即可** **自动注册 NPC 为系统服务**。只有需要 **多开** 才需手动管理多个实例。

#### **Linux（Systemd）**
📌 **自动安装的服务文件为 `Npc.service`**
创建 `systemd` 配置文件（路径：`/etc/systemd/system/npc.service`）：
```ini
[Unit]
Description=NPS 内网穿透客户端
ConditionFileIsExecutable=/usr/bin/npc
Requires=network.target
After=network-online.target syslog.target

[Service]
LimitNOFILE=65536
StartLimitInterval=5
StartLimitBurst=10
ExecStart=/usr/bin/npc "-server=xxx:123" "-vkey=xxx" "-type=tcp" "-debug=false" "-log=off"
Restart=always
RestartSec=120

[Install]
WantedBy=multi-user.target
```
**启用并启动服务**
```bash
systemctl enable npc
systemctl start npc
```
📌 **卸载 NPC 服务**
```bash
systemctl stop npc
systemctl disable npc
rm /etc/systemd/system/npc.service
systemctl daemon-reload
```
> **不会使用 `systemctl`？** 请参考 [Systemd 官方文档](https://docs.redhat.com/zh-cn/documentation/red_hat_enterprise_linux/9/html/configuring_basic_system_settings/managing-system-services-with-systemctl_managing-systemd#starting-a-system-service_managing-system-services-with-systemctl)。

---

#### **Windows（SC 命令）**
📌 **Windows 手动注册服务**
以 **管理员身份** 运行 `PowerShell`：
```powershell
cmd /c 'sc create Npc1 binPath= "D:\tools\npc.exe -server=xxx:123 -vkey=xxx -type=tcp -tls_enable=true -log=off -debug=false" DisplayName= "nps内网穿透客户端1" start= auto'
```
**启动服务**
```powershell
sc start Npc1
```
**删除服务**
```powershell
sc stop Npc1
sc delete Npc1
```
> **Windows 注册系统服务后，如需更新，必须先手动停止所有运行的服务。**
> 
> **[微软SC命令指南](https://learn.microsoft.com/zh-cn/windows-server/administration/windows-commands/sc-create)**

---

## 3. 多开客户端（多个 NPC 实例）

### **3.1 Windows 多开**
```powershell
cmd /c 'sc create Npc2 binPath= "D:\tools\npc.exe -server=xxx:123 -vkey=yyy -type=tcp -tls_enable=true -log=off -debug=false" DisplayName= "NPS Client 2" start= auto'
```
**启用并启动服务**
```powershell
sc start Npc2
```
📌 **删除多开服务**
```powershell
sc stop Npc2
sc delete Npc2
```
> 📌 **多个客户端实例** 需要 **不同的 `vkey`**。

---

### **3.2 Linux 多开**
**创建 `systemd` 配置文件**，例如 `/etc/systemd/system/npc2.service`：
```ini
[Unit]
Description=NPS 内网穿透客户端 2
ConditionFileIsExecutable=/usr/bin/npc
Requires=network.target
After=network-online.target syslog.target

[Service]
LimitNOFILE=65536
StartLimitInterval=5
StartLimitBurst=10
ExecStart=/usr/bin/npc "-server=xxx:123" "-vkey=yyy" "-type=tcp" "-debug=false" "-log=off"
Restart=always
RestartSec=120

[Install]
WantedBy=multi-user.target
```
**启用并启动**
```bash
systemctl enable npc2
systemctl start npc2
```

---

## 4. 版本检查
- 服务器端版本：
  ```bash
  nps -version
  ```
- 客户端版本：
  ```bash
  npc -version
  ```
  
---

## **5. 配置管理**
- **客户端连接后，在 Web 界面配置穿透服务**
- 参考 [使用示例](/example)

