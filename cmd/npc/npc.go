package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/client"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/config"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/install"
	"ehang.io/nps/lib/version"
	"github.com/astaxie/beego/logs"
	"github.com/ccding/go-stun/stun"
	"github.com/kardianos/service"
)

// 全局配置变量
var (
	serverAddr     = flag.String("server", "", "Server addr (ip:port)")
	configPath     = flag.String("config", "", "Configuration file path")
	verifyKey      = flag.String("vkey", "", "Authentication key")
	logType        = flag.String("log", "stdout", "Log output mode (stdout|file)")
	connType       = flag.String("type", "tcp", "Connection type with the server (kcp|tcp)")
	proxyUrl       = flag.String("proxy", "", "Proxy socks5 URL (eg: socks5://111:222@127.0.0.1:9007)")
	logLevel       = flag.String("log_level", "7", "Log level 0~7")
	registerTime   = flag.Int("time", 2, "Register time in hours")
	localPort      = flag.Int("local_port", 2000, "P2P local port")
	password       = flag.String("password", "", "P2P password flag")
	target         = flag.String("target", "", "P2P target")
	localType      = flag.String("local_type", "p2p", "P2P target type")
	logPath        = flag.String("log_path", "", "NPC log path (empty to use default)")
	logMaxSize     = flag.Int("log_max_size", 10, "Maximum log file size in MB before rotation")
	logMaxDays     = flag.Int("log_max_days", 7, "Number of days to retain old log files")
	logDaily       = flag.Bool("log_daily", false, "Rotate log daily")
	debug          = flag.Bool("debug", true, "Enable debug mode")
	pprofAddr      = flag.String("pprof", "", "PProf debug address (ip:port)")
	stunAddr       = flag.String("stun_addr", "stun.stunprotocol.org:3478", "STUN server address")
	ver            = flag.Bool("version", false, "Show current version")
	disconnectTime = flag.Int("disconnect_timeout", 60, "Disconnect timeout in seconds")
	tlsEnable      = flag.Bool("tls_enable", false, "Enable TLS")
)

func main() {
	flag.Parse()
	logs.Reset()
	logs.EnableFuncCallDepth(true)
	logs.SetLogFuncCallDepth(3)

	// 显示版本并退出
	if *ver {
		common.PrintVersion()
		return
	}

	// 配置日志
	configureLogging()

	// 初始化服务
	initService()
}

// 配置日志记录
func configureLogging() {
	// 处理日志路径默认值
	if *logPath == "" {
		*logPath = common.GetNpcLogPath() // 使用默认路径
	} else if strings.EqualFold(*logPath, "off") || strings.EqualFold(*logPath, "false") || strings.EqualFold(*logPath, "/dev/null") {
		*logPath = "" // 禁用文件日志输出
	}

	// 针对 Windows 系统调整日志路径中的反斜杠
	if common.IsWindows() && *logPath != "" {
		*logPath = strings.Replace(*logPath, "\\", "\\\\", -1)
	}

	// 仅启用控制台日志（如果 logPath 被禁用）
	if *logPath == "" {
		if *debug {
			logs.SetLogger(logs.AdapterConsole, `{"level":`+*logLevel+`,"color":true}`)
		}
		return
	}

	// 设置文件日志，按大小和天数滚动
	if strings.EqualFold(*logType, "file") {
		logs.SetLogger(logs.AdapterFile, `{
			"filename":"`+*logPath+`",
			"level":`+*logLevel+`,
			"daily":`+fmt.Sprintf("%v", *logDaily)+`,
			"maxsize":`+fmt.Sprintf("%d", *logMaxSize*1024*1024)+`, 
			"maxdays":`+fmt.Sprintf("%d", *logMaxDays)+`
		}`)
	} else { // 默认控制台日志
		logs.SetLogger(logs.AdapterConsole, `{"level":`+*logLevel+`,"color":true}`)
	}
}

// 初始化服务
func initService() {
	options := make(service.KeyValue)
	svcConfig := &service.Config{
		Name:        "Npc",
		DisplayName: "nps内网穿透客户端",
		Description: "一款轻量级、功能强大的内网穿透代理服务器。支持tcp、udp流量转发，支持内网http代理、内网socks5代理，同时支持snappy压缩、站点保护、加密传输、多路复用、header修改等。支持web图形化管理，集成多用户模式。",
		Option:      options,
	}

	// 非 Windows 系统添加服务依赖
	if !common.IsWindows() {
		svcConfig.Dependencies = []string{
			"Requires=network.target",
			"After=network-online.target syslog.target"}
		svcConfig.Option["SystemdScript"] = install.SystemdScript
		svcConfig.Option["SysvScript"] = install.SysvScript
	}

	// 配置服务启动参数
	for _, v := range os.Args[1:] {
		switch v {
		case "install", "start", "stop", "uninstall", "restart":
			continue
		}
		if !strings.Contains(v, "-service=") && !strings.Contains(v, "-debug=") {
			svcConfig.Arguments = append(svcConfig.Arguments, v)
		}
	}
	svcConfig.Arguments = append(svcConfig.Arguments, "-debug=false")

	// 创建服务
	prg := &npc{
		exit: make(chan struct{}),
	}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		logs.Error(err, "service function disabled")
		runWithoutService()
		return
	}

	// 处理服务命令
	handleServiceCommands(s)
	s.Run()
}

// 无服务运行
func runWithoutService() {
	run()
	wg := sync.WaitGroup{}
	wg.Add(1)
	wg.Wait()
}

// 处理服务相关命令
func handleServiceCommands(s service.Service) {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "status":
			if len(os.Args) > 2 {
				path := strings.Replace(os.Args[2], "-config=", "", -1)
				client.GetTaskStatus(path)
			}
		case "register":
			flag.CommandLine.Parse(os.Args[2:])
			client.RegisterLocalIp(*serverAddr, *verifyKey, *connType, *proxyUrl, *registerTime)
		case "update":
			install.UpdateNpc()
			return
		case "nat":
			handleNAT()
		case "start", "stop", "restart":
			handleUnixServiceCommands(s)
		case "install":
			installAndStartService(s)
		case "uninstall":
			uninstallService(s)
		}
	}
}

// NAT 处理
func handleNAT() {
	c := stun.NewClient()
	flag.CommandLine.Parse(os.Args[2:])
	c.SetServerAddr(*stunAddr)
	nat, host, err := c.Discover()
	if err != nil || host == nil {
		logs.Error("get nat type error", err)
		return
	}
	fmt.Printf("nat type: %s \npublic address: %s\n", nat.String(), host.String())
	os.Exit(0)
}

// Unix 服务命令
func handleUnixServiceCommands(s service.Service) {
	if service.Platform() == "unix-systemv" {
		logs.Info("unix-systemv service")
		cmd := exec.Command("/etc/init.d/"+s.String(), os.Args[1])
		if err := cmd.Run(); err != nil {
			logs.Error(err)
		}
		return
	}
	if err := service.Control(s, os.Args[1]); err != nil {
		logs.Error("Valid actions: %q\n%s", service.ControlAction, err.Error())
	}
}

// 安装并启动服务
func installAndStartService(s service.Service) {
	service.Control(s, "stop")
	service.Control(s, "uninstall")
	install.InstallNpc()
	if err := service.Control(s, os.Args[1]); err != nil {
		logs.Error("Valid actions: %q\n%s", service.ControlAction, err.Error())
	}
	if service.Platform() == "unix-systemv" {
		logs.Info("unix-systemv service")
		confPath := "/etc/init.d/" + s.String()
		os.Symlink(confPath, "/etc/rc.d/S90"+s.String())
		os.Symlink(confPath, "/etc/rc.d/K02"+s.String())
	}
}

// 卸载服务
func uninstallService(s service.Service) {
	if err := service.Control(s, os.Args[1]); err != nil {
		logs.Error("Valid actions: %q\n%s", service.ControlAction, err.Error())
	}
	if service.Platform() == "unix-systemv" {
		logs.Info("unix-systemv service")
		os.Remove("/etc/rc.d/S90" + s.String())
		os.Remove("/etc/rc.d/K02" + s.String())
	}
}

type npc struct {
	exit chan struct{}
}

func (p *npc) Start(s service.Service) error {
	go p.run()
	return nil
}

func (p *npc) Stop(s service.Service) error {
	close(p.exit)
	if service.Interactive() {
		os.Exit(0)
	}
	return nil
}

func (p *npc) run() error {
	defer func() {
		if err := recover(); err != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			logs.Warning("npc: panic serving %v: %v\n%s", err, string(buf))
		}
	}()
	run()
	select {
	case <-p.exit:
		logs.Warning("stop...")
	}
	return nil
}

// 主运行逻辑
func run() {
	common.InitPProfFromArg(*pprofAddr)
	//p2p or secret command
	if *password != "" {
		commonConfig := new(config.CommonConfig)
		commonConfig.Server = *serverAddr
		commonConfig.VKey = *verifyKey
		commonConfig.Tp = *connType
		localServer := new(config.LocalServer)
		localServer.Type = *localType
		localServer.Password = *password
		localServer.Target = *target
		localServer.Port = *localPort
		commonConfig.Client = new(file.Client)
		commonConfig.Client.Cnf = new(file.Config)
		go client.StartLocalServer(localServer, commonConfig)
		return
	}
	env := common.GetEnvMap()
	if *serverAddr == "" {
		*serverAddr, _ = env["NPC_SERVER_ADDR"]
	}
	if *verifyKey == "" {
		*verifyKey, _ = env["NPC_SERVER_VKEY"]
	}
	if *verifyKey != "" && *serverAddr != "" && *configPath == "" {
		client.SetTlsEnable(*tlsEnable)
		logs.Info("the version of client is %s, the core version of client is %s, tls enable is %t", version.VERSION, version.GetVersion(), client.GetTlsEnable())

		vkeys := strings.Split(*verifyKey, `,`)
		for _, key := range vkeys {
			key := key
			go func() {
				for {
					logs.Info("start vkey:" + key)
					client.NewRPClient(*serverAddr, key, *connType, *proxyUrl, nil, *disconnectTime).Start()
					logs.Info("Client closed! It will be reconnected in five seconds")
					time.Sleep(time.Second * 5)
				}
			}()
		}
	} else {
		if *configPath == "" {
			*configPath = common.GetConfigPath()
		}
		go client.StartFromFile(*configPath)
	}
}
