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

	"github.com/beego/beego/logs"
	"github.com/ccding/go-stun/stun"
	"github.com/djylb/nps/client"
	"github.com/djylb/nps/lib/common"
	"github.com/djylb/nps/lib/config"
	"github.com/djylb/nps/lib/file"
	"github.com/djylb/nps/lib/install"
	"github.com/djylb/nps/lib/version"
	"github.com/kardianos/service"
)

// 全局配置变量
var (
	serverAddr     = flag.String("server", "", "Server addr (ip:port)")
	configPath     = flag.String("config", "", "Configuration file path")
	verifyKey      = flag.String("vkey", "", "Authentication key")
	logType        = flag.String("log", "file", "Log output mode (stdout|file|both|off)")
	connType       = flag.String("type", "tcp", "Connection type with the server (kcp|tcp|tls)")
	proxyUrl       = flag.String("proxy", "", "Proxy socks5 URL (eg: socks5://user:pass@127.0.0.1:9007)")
	logLevel       = flag.String("log_level", "7", "Log level 0~7")
	registerTime   = flag.Int("time", 2, "Register time in hours")
	localPort      = flag.Int("local_port", 2000, "P2P local port")
	password       = flag.String("password", "", "P2P password flag")
	target         = flag.String("target", "", "P2P target")
	localType      = flag.String("local_type", "p2p", "P2P target type")
	logPath        = flag.String("log_path", "", "NPC log path (empty to use default, 'off' to disable)")
	logMaxSize     = flag.Int("log_max_size", 5, "Maximum log file size in MB before rotation (0 to disable)")
	logMaxDays     = flag.String("log_max_days", "7", "Number of days to retain old log files (0 to disable)")
	logMaxFiles    = flag.String("log_max_files", "10", "Maximum number of log files to retain (0 to disable)")
	logDaily       = flag.String("log_daily", "true", "Rotate log daily (true or false)")
	debug          = flag.Bool("debug", true, "Enable debug mode")
	pprofAddr      = flag.String("pprof", "", "PProf debug address (ip:port)")
	stunAddr       = flag.String("stun_addr", "stun.miwifi.com:3478", "STUN server address")
	ver            = flag.Bool("version", false, "Show current version")
	disconnectTime = flag.Int("disconnect_timeout", 60, "Disconnect timeout in seconds")
	dnsServer      = flag.String("dns_server", "8.8.8.8", "DNS server for domain lookup")
	tlsEnable      = flag.Bool("tls_enable", false, "Enable TLS (Deprecated)")
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

	// 配置DNS
	common.SetCustomDNS(*dnsServer)

	// 初始化服务
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
		run()
		// run without service
		wg := sync.WaitGroup{}
		wg.Add(1)
		wg.Wait()
		return
	}

	// 处理服务命令
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
			c := stun.NewClient()
			flag.CommandLine.Parse(os.Args[2:])
			c.SetServerAddr(*stunAddr)
			logs.Info(*stunAddr)
			nat, host, err := c.Discover()
			if err != nil {
				logs.Error("Error:", err)
				return
			}
			fmt.Println("NAT Type:", nat)
			if host != nil {
				fmt.Println("External IP Family:", host.Family())
				fmt.Println("External IP:", host.IP())
				fmt.Println("External Port:", host.Port())
			}
			os.Exit(0)
		case "start", "stop", "restart":
			// support busyBox and sysV, for openWrt
			if service.Platform() == "unix-systemv" {
				logs.Info("unix-systemv service")
				cmd := exec.Command("/etc/init.d/"+svcConfig.Name, os.Args[1])
				err := cmd.Run()
				if err != nil {
					logs.Error(err)
				}
				return
			}
			err := service.Control(s, os.Args[1])
			if err != nil {
				logs.Error("Valid actions: %q\n%s", service.ControlAction, err.Error())
			}
			return
		case "install":
			service.Control(s, "stop")
			service.Control(s, "uninstall")
			install.InstallNpc()
			err := service.Control(s, os.Args[1])
			if err != nil {
				logs.Error("Valid actions: %q\n%s", service.ControlAction, err.Error())
			}
			if service.Platform() == "unix-systemv" {
				logs.Info("unix-systemv service")
				confPath := "/etc/init.d/" + svcConfig.Name
				os.Symlink(confPath, "/etc/rc.d/S90"+svcConfig.Name)
				os.Symlink(confPath, "/etc/rc.d/K02"+svcConfig.Name)
			}
			return
		case "uninstall":
			err := service.Control(s, os.Args[1])
			if err != nil {
				logs.Error("Valid actions: %q\n%s", service.ControlAction, err.Error())
			}
			if service.Platform() == "unix-systemv" {
				logs.Info("unix-systemv service")
				os.Remove("/etc/rc.d/S90" + svcConfig.Name)
				os.Remove("/etc/rc.d/K02" + svcConfig.Name)
			}
			return
		}
	}
	s.Run()
}

// 配置日志记录
func configureLogging() {
	// 关闭日志输出
	if strings.EqualFold(*logType, "off") || strings.EqualFold(*logType, "false") {
		if common.IsWindows() {
			logs.SetLogger(logs.AdapterFile, `{"filename":"NUL"}`)
		} else {
			logs.SetLogger(logs.AdapterFile, `{"filename":"/dev/null"}`)
		}
		return
	}

	// 控制台日志
	if *debug {
		logs.SetLogger(logs.AdapterConsole, `{"level":7,"color":true}`)
	} else if strings.EqualFold(*logType, "stdout") || strings.EqualFold(*logType, "both") {
		logs.SetLogger(logs.AdapterConsole, `{"level":`+*logLevel+`,"color":true}`)
	}

	// 处理日志路径默认值
	if *logPath == "" {
		*logPath = common.GetNpcLogPath() // 使用默认路径
	} else if strings.EqualFold(*logPath, "off") || strings.EqualFold(*logPath, "false") {
		if common.IsWindows() {
			logs.SetLogger(logs.AdapterFile, `{"filename":"NUL"}`)
		} else {
			logs.SetLogger(logs.AdapterFile, `{"filename":"/dev/null"}`)
		}
	}

	// 针对 Windows 系统调整日志路径中的反斜杠
	if common.IsWindows() {
		*logPath = strings.Replace(*logPath, "\\", "\\\\", -1)
	}

	// 设置文件日志，按大小、天数和文件数量滚动
	if strings.EqualFold(*logType, "file") || strings.EqualFold(*logType, "both") {
		logs.SetLogger(logs.AdapterFile, `{"level":`+*logLevel+`,"filename":"`+*logPath+`","daily":`+*logDaily+`,"maxfiles":`+*logMaxFiles+`,"maxdays":`+*logMaxDays+`,"maxsize":`+fmt.Sprintf("%d", *logMaxSize*1024*1024)+`,"maxlines":100000,"rotate":true,"color":true}`)
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
	if *tlsEnable {
		*connType = "tls"
	}
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
		logs.Info("the version of client is %s, the core version of client is %s", version.VERSION, version.GetVersion())
		*serverAddr = strings.ReplaceAll(*serverAddr, "，", ",")
		*verifyKey = strings.ReplaceAll(*verifyKey, "，", ",")
		*connType = strings.ReplaceAll(*connType, "，", ",")

		serverAddrs := strings.Split(*serverAddr, ",")
		verifyKeys := strings.Split(*verifyKey, ",")
		connTypes := strings.Split(*connType, ",")

		for i := 0; i < len(serverAddrs); i++ {
			serverAddrs[i] = strings.TrimSpace(serverAddrs[i])
			if i > 0 && serverAddrs[i] == "" {
				serverAddrs[i] = serverAddrs[i-1]
			}
		}
		for i := 0; i < len(verifyKeys); i++ {
			verifyKeys[i] = strings.TrimSpace(verifyKeys[i])
			if i > 0 && verifyKeys[i] == "" {
				verifyKeys[i] = verifyKeys[i-1]
			}
		}
		for i := 0; i < len(connTypes); i++ {
			connTypes[i] = strings.TrimSpace(connTypes[i])
			if i > 0 && connTypes[i] == "" {
				connTypes[i] = connTypes[i-1]
			}
		}

		for len(serverAddrs) > 0 && serverAddrs[len(serverAddrs)-1] == "" {
			serverAddrs = serverAddrs[:len(serverAddrs)-1]
		}
		for len(verifyKeys) > 0 && verifyKeys[len(verifyKeys)-1] == "" {
			verifyKeys = verifyKeys[:len(verifyKeys)-1]
		}
		for len(connTypes) > 0 && connTypes[len(connTypes)-1] == "" {
			connTypes = connTypes[:len(connTypes)-1]
		}

		if len(connTypes) == 0 {
			connTypes = append(connTypes, "tcp")
		}

		if len(serverAddrs) == 0 || len(verifyKeys) == 0 {
			logs.Error("serverAddr or verifyKey cannot be empty")
			return
		}

		maxLength := len(serverAddrs)
		if len(verifyKeys) > maxLength {
			maxLength = len(verifyKeys)
		}
		if len(connTypes) > maxLength {
			maxLength = len(connTypes)
		}

		for len(serverAddrs) < maxLength {
			serverAddrs = append(serverAddrs, serverAddrs[len(serverAddrs)-1])
		}
		for len(verifyKeys) < maxLength {
			verifyKeys = append(verifyKeys, verifyKeys[len(verifyKeys)-1])
		}
		for len(connTypes) < maxLength {
			connTypes = append(connTypes, connTypes[len(connTypes)-1])
		}

		for i := 0; i < maxLength; i++ {
			serverAddr := serverAddrs[i]
			verifyKey := verifyKeys[i]
			connType := connTypes[i]

			go func() {
				for {
					logs.Info("Start server: " + serverAddr + " vkey: " + verifyKey + " type: " + connType)
					client.NewRPClient(serverAddr, verifyKey, connType, *proxyUrl, nil, *disconnectTime).Start()
					logs.Info("Client closed! It will be reconnected in five seconds")
					time.Sleep(time.Second * 5)
				}
			}()
		}
	} else {
		if *configPath == "" {
			*configPath = common.GetConfigPath()
		}

		configPaths := strings.Split(*configPath, ",")
		for i := range configPaths {
			configPaths[i] = strings.TrimSpace(configPaths[i])
		}

		for _, path := range configPaths {
			go client.StartFromFile(path)
		}
	}
}
