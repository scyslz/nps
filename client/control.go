package client

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/djylb/nps/lib/common"
	"github.com/djylb/nps/lib/config"
	"github.com/djylb/nps/lib/conn"
	"github.com/djylb/nps/lib/crypt"
	"github.com/djylb/nps/lib/logs"
	"github.com/djylb/nps/lib/version"
	"github.com/xtaci/kcp-go/v5"
	"golang.org/x/net/proxy"
)

var Ver = version.GetLatestIndex()

func GetTaskStatus(path string) {
	cnf, err := config.NewConfig(path)
	if err != nil {
		log.Fatalln(err)
	}
	c, err := NewConn(cnf.CommonConfig.Tp, cnf.CommonConfig.VKey, cnf.CommonConfig.Server, common.WORK_CONFIG, cnf.CommonConfig.ProxyUrl)
	if err != nil {
		log.Fatalln(err)
	}
	if _, err := c.Write([]byte(common.WORK_STATUS)); err != nil {
		log.Fatalln(err)
	}
	//read now vKey and write to server
	if f, err := common.ReadAllFromFile(filepath.Join(common.GetTmpPath(), "npc_vkey.txt")); err != nil {
		log.Fatalln(err)
	} else if _, err := c.Write([]byte(crypt.Md5(string(f)))); err != nil {
		log.Fatalln(err)
	}
	var isPub bool
	binary.Read(c, binary.LittleEndian, &isPub)
	if l, err := c.GetLen(); err != nil {
		log.Fatalln(err)
	} else if b, err := c.GetShortContent(l); err != nil {
		log.Fatalln(err)
	} else {
		arr := strings.Split(string(b), common.CONN_DATA_SEQ)
		for _, v := range cnf.Hosts {
			if common.InStrArr(arr, v.Remark) {
				log.Println(v.Remark, "ok")
			} else {
				log.Println(v.Remark, "not running")
			}
		}
		for _, v := range cnf.Tasks {
			ports := common.GetPorts(v.Ports)
			if v.Mode == "secret" {
				ports = append(ports, 0)
			}
			for _, vv := range ports {
				var remark string
				if len(ports) > 1 {
					remark = v.Remark + "_" + strconv.Itoa(vv)
				} else {
					remark = v.Remark
				}
				if common.InStrArr(arr, remark) {
					log.Println(remark, "ok")
				} else {
					log.Println(remark, "not running")
				}
			}
		}
	}
	os.Exit(0)
}

var errAdd = errors.New("The server returned an error, which port or host may have been occupied or not allowed to open.")

func StartFromFile(path string) {
	first := true
	cnf, err := config.NewConfig(path)
	if err != nil || cnf.CommonConfig == nil {
		logs.Error("Config file %s loading error %v", path, err)
		os.Exit(0)
	}
	logs.Info("Loading configuration file %s successfully", path)

	common.SetCustomDNS(cnf.CommonConfig.DnsServer)

	logs.Info("the version of client is %s, the core version of client is %s", version.VERSION, version.GetLatest())

	for {
		if !first && !cnf.CommonConfig.AutoReconnection {
			return
		}
		if !first {
			logs.Info("Reconnecting...")
			time.Sleep(time.Second * 5)
		}
		first = false
		if cnf.CommonConfig.TlsEnable {
			cnf.CommonConfig.Tp = "tls"
		}
		// Fetch latest server URL from subscription if provided
		if cnf.CommonConfig.SubsriptionServer != "" {
			if updatedServer, err := fetchServerFromSubscription(cnf.CommonConfig.SubsriptionServer); err == nil {
				logs.Info("Successfully fetched latest server from subscription: %s", updatedServer)
				cnf.CommonConfig.Server = updatedServer
			} else {
				logs.Error("Failed to fetch latest server from subscription: %v. Using configured server: %s", err, cnf.CommonConfig.Server)
			}
		}
		c, err := NewConn(cnf.CommonConfig.Tp, cnf.CommonConfig.VKey, cnf.CommonConfig.Server, common.WORK_CONFIG, cnf.CommonConfig.ProxyUrl)
		if err != nil {
			logs.Error("%v", err)
			continue
		}

		var isPub bool
		binary.Read(c, binary.LittleEndian, &isPub)

		// get tmp password
		var b []byte
		vkey := cnf.CommonConfig.VKey
		if isPub {
			// send global configuration to server and get status of config setting
			if _, err := c.SendInfo(cnf.CommonConfig.Client, common.NEW_CONF); err != nil {
				logs.Error("%v", err)
				continue
			}
			if !c.GetAddStatus() {
				logs.Error("the web_user may have been occupied!")
				continue
			}

			if b, err = c.GetShortContent(16); err != nil {
				logs.Error("%v", err)
				continue
			}
			vkey = string(b)
		}

		if err := ioutil.WriteFile(filepath.Join(common.GetTmpPath(), "npc_vkey.txt"), []byte(vkey), 0600); err != nil {
			logs.Debug("Failed to write vkey file: %v", err)
			//continue
		}

		//send hosts to server
		for _, v := range cnf.Hosts {
			if _, err := c.SendInfo(v, common.NEW_HOST); err != nil {
				logs.Error("%v", err)
				continue
			}
			if !c.GetAddStatus() {
				logs.Error("%v %s", errAdd, v.Host)
				continue
			}
		}

		//send  task to server
		for _, v := range cnf.Tasks {
			if _, err := c.SendInfo(v, common.NEW_TASK); err != nil {
				logs.Error("%v", err)
				continue
			}
			if !c.GetAddStatus() {
				logs.Error("%v %s %s", errAdd, v.Ports, v.Remark)
				continue
			}
			if v.Mode == "file" {
				//start local file server
				go startLocalFileServer(cnf.CommonConfig, v, vkey)
			}
		}

		//create local server secret or p2p
		for _, v := range cnf.LocalServer {
			go StartLocalServer(v, cnf.CommonConfig)
		}

		c.Close()
		if cnf.CommonConfig.Client.WebUserName == "" || cnf.CommonConfig.Client.WebPassword == "" {
			logs.Info("web access login username:user password:%s", vkey)
		} else {
			logs.Info("web access login username:%s password:%s", cnf.CommonConfig.Client.WebUserName, cnf.CommonConfig.Client.WebPassword)
		}

		NewRPClient(cnf.CommonConfig.Server, vkey, cnf.CommonConfig.Tp, cnf.CommonConfig.ProxyUrl, cnf, cnf.CommonConfig.DisconnectTime).Start()
		CloseLocalServer()
	}
}

// Fetches the latest server URL from a subscription address
func fetchServerFromSubscription(subAddr string) (string, error) {
	resp, err := http.Get(subAddr)
	if err != nil {
		return "", fmt.Errorf("failed to perform HTTP GET request to subscription address: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body from subscription address: %v", err)
	}
	return strings.TrimSpace(string(body)), nil
}

// Create a new connection with the server and verify it
func NewConn(tp string, vkey string, server string, connType string, proxyUrl string) (*conn.Conn, error) {
	//logs.Debug("NewConn: %s %s %s %s %s", tp, vkey, server, connType, proxyUrl)
	var err error
	var connection net.Conn
	var sess *kcp.UDPSession

	timeout := time.Second * 10
	dialer := net.Dialer{Timeout: timeout}
	path := "/"
	server, path = common.SplitServerAndPath(server)
	logs.Debug("Server: %s Path: %s", server, path)
	server, err = common.GetFastAddr(server, tp)
	if err != nil {
		logs.Debug("Fast Server: %s Path: %s Error: %v", server, path, err)
	} else {
		logs.Debug("Fast Server: %s Path: %s", server, path)
	}

	if tp == "tcp" || tp == "tls" || tp == "ws" || tp == "wss" {
		var rawConn net.Conn

		if proxyUrl != "" {
			u, er := url.Parse(proxyUrl)
			if er != nil {
				return nil, er
			}
			switch u.Scheme {
			case "socks5":
				n, er := proxy.FromURL(u, nil)
				if er != nil {
					return nil, er
				}
				rawConn, err = n.Dial("tcp", server)
			default:
				rawConn, err = NewHttpProxyConn(u, server)
			}
		} else {
			rawConn, err = dialer.Dial("tcp", server)
		}
		if err != nil {
			return nil, err
		}
		if tp == "tls" {
			//logs.Debug("GetTls")
			conf := &tls.Config{InsecureSkipVerify: true}
			connection, err = conn.NewTlsConn(rawConn, timeout, conf)
		} else {
			connection = rawConn
		}
	} else {
		sess, err = kcp.DialWithOptions(server, nil, 10, 3)
		if err == nil {
			conn.SetUdpSession(sess)
			connection = sess
		}
	}

	if err != nil {
		return nil, err
	}

	//logs.Debug("SetDeadline")
	connection.SetDeadline(time.Now().Add(timeout))
	defer connection.SetDeadline(time.Time{}) // 解除超时限制

	c := conn.NewConn(connection)
	if _, err := c.Write([]byte(common.CONN_TEST)); err != nil {
		return nil, err
	}
	minVerBytes := []byte(version.GetVersion(Ver))
	if err := c.WriteLenContent(minVerBytes); err != nil {
		return nil, err
	}
	vs := []byte(version.VERSION)
	if err := c.WriteLenContent(vs); err != nil {
		return nil, err
	}

	if Ver == 0 {
		// 0.26.0
		b, err := c.GetShortContent(32)
		if err != nil {
			logs.Error("%v", err)
			return nil, err
		}
		if crypt.Md5(version.GetVersion(Ver)) != string(b) {
			logs.Warn("The client does not match the server version. The current core version of the client is", version.GetVersion(Ver))
			//return nil, err
		}
		if _, err := c.Write([]byte(crypt.Md5(vkey))); err != nil {
			return nil, err
		}
		if s, err := c.ReadFlag(); err != nil {
			return nil, err
		} else if s == common.VERIFY_EER {
			return nil, errors.New(fmt.Sprintf("Validation key %s incorrect", vkey))
		}
		if _, err := c.Write([]byte(connType)); err != nil {
			return nil, err
		}
	} else {
		// 0.27.0
		ts := time.Now().Unix() - int64(rand.Intn(6))
		if _, err := c.Write(common.TimestampToBytes(ts)); err != nil {
			return nil, err
		}
		if _, err := c.Write([]byte(crypt.Blake2b(vkey))); err != nil {
			return nil, err
		}
		ipBuf, err := crypt.EncryptBytes(common.EncodeIP(common.GetOutboundIP()), vkey)
		if err := c.WriteLenContent(ipBuf); err != nil {
			return nil, err
		}
		randBuf, err := common.RandomBytes(1000)
		if err != nil {
			return nil, err
		}
		if err := c.WriteLenContent(randBuf); err != nil {
			return nil, err
		}
		if _, err := c.Write(crypt.ComputeHMAC(vkey, ts, minVerBytes, vs, ipBuf, randBuf)); err != nil {
			return nil, err
		}
		b, err := c.GetShortContent(32)
		if err != nil {
			logs.Error("%v", err)
			return nil, errors.New(fmt.Sprintf("Validation key %s incorrect", vkey))
		}
		if crypt.Md5(version.GetVersion(Ver)) != string(b) {
			logs.Warn("The client does not match the server version. The current core version of the client is", version.GetVersion(Ver))
			return nil, err
		}
		if _, err := c.Write([]byte(connType)); err != nil {
			return nil, err
		}
	}

	c.SetAlive()

	return c, nil
}

// http proxy connection
func NewHttpProxyConn(url *url.URL, remoteAddr string) (net.Conn, error) {
	req, err := http.NewRequest("CONNECT", "http://"+remoteAddr, nil)
	if err != nil {
		return nil, err
	}
	password, _ := url.User.Password()
	req.Header.Set("Authorization", "Basic "+basicAuth(strings.Trim(url.User.Username(), " "), password))
	// we make a http proxy request
	proxyConn, err := net.Dial("tcp", url.Host)
	if err != nil {
		return nil, err
	}
	if err := req.Write(proxyConn); err != nil {
		return nil, err
	}
	res, err := http.ReadResponse(bufio.NewReader(proxyConn), req)
	if err != nil {
		return nil, err
	}
	_ = res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("Proxy error " + res.Status)
	}
	return proxyConn, nil
}

// get a basic auth string
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func getRemoteAddressFromServer(rAddr string, localConn *net.UDPConn, md5Password, role string, add int) error {
	rAddr, err := getNextAddr(rAddr, add)
	if err != nil {
		logs.Error("%v", err)
		return err
	}
	addr, err := net.ResolveUDPAddr("udp", rAddr)
	if err != nil {
		return err
	}
	if _, err := localConn.WriteTo(common.GetWriteStr(md5Password, role), addr); err != nil {
		return err
	}
	return nil
}

func handleP2PUdp(localAddr, rAddr, md5Password, role string) (remoteAddress string, c net.PacketConn, err error) {
	localConn, err := newUdpConnByAddr(localAddr)
	if err != nil {
		return
	}
	err = getRemoteAddressFromServer(rAddr, localConn, md5Password, role, 0)
	if err != nil {
		logs.Error("%v", err)
		return
	}
	err = getRemoteAddressFromServer(rAddr, localConn, md5Password, role, 1)
	if err != nil {
		logs.Error("%v", err)
		return
	}
	err = getRemoteAddressFromServer(rAddr, localConn, md5Password, role, 2)
	if err != nil {
		logs.Error("%v", err)
		return
	}
	var remoteAddr1, remoteAddr2, remoteAddr3 string
	for {
		buf := make([]byte, 1024)
		if n, addr, er := localConn.ReadFromUDP(buf); er != nil {
			err = er
			return
		} else {
			rAddr2, _ := getNextAddr(rAddr, 1)
			rAddr3, _ := getNextAddr(rAddr, 2)
			switch addr.String() {
			case rAddr:
				remoteAddr1 = string(buf[:n])
			case rAddr2:
				remoteAddr2 = string(buf[:n])
			case rAddr3:
				remoteAddr3 = string(buf[:n])
			}
		}
		//logs.Debug("buf: %s", buf)
		logs.Debug("remoteAddr1: %s remoteAddr2: %s remoteAddr3: %s", remoteAddr1, remoteAddr2, remoteAddr3)
		if remoteAddr1 != "" && remoteAddr2 != "" && remoteAddr3 != "" {
			break
		}
	}
	if remoteAddress, err = sendP2PTestMsg(localConn, remoteAddr1, remoteAddr2, remoteAddr3); err != nil {
		return
	}
	c, err = newUdpConnByAddr(localAddr)
	return
}

func sendP2PTestMsg(localConn *net.UDPConn, remoteAddr1, remoteAddr2, remoteAddr3 string) (string, error) {
	logs.Trace("%s %s %s", remoteAddr3, remoteAddr2, remoteAddr1)
	defer localConn.Close()
	isClose := false
	defer func() { isClose = true }()
	interval, err := getAddrInterval(remoteAddr1, remoteAddr2, remoteAddr3)
	if err != nil {
		return "", err
	}
	go func() {
		addr, err := getNextAddr(remoteAddr3, interval)
		if err != nil {
			return
		}
		remoteUdpAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return
		}
		logs.Trace("try send test packet to target %s", addr)
		ticker := time.NewTicker(time.Millisecond * 500)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if isClose {
					return
				}
				if _, err := localConn.WriteTo([]byte(common.WORK_P2P_CONNECT), remoteUdpAddr); err != nil {
					return
				}
			}
		}
	}()
	if interval != 0 {
		ip := common.RemovePortFromHost(remoteAddr2)
		p1 := common.GetPortByAddr(remoteAddr1)
		p2 := common.GetPortByAddr(remoteAddr2)
		p3 := common.GetPortByAddr(remoteAddr3)
		go func() {
			startPort := p3
			endPort := startPort + (interval * 50)
			if (p1 < p3 && p3 < p2) || (p1 > p3 && p3 > p2) {
				endPort = endPort + (p2 - p3)
			}
			endPort = common.GetPort(endPort)
			logs.Debug("Start Port: %d, End Port: %d, Interval: %d", startPort, endPort, interval)
			ports := getRandomPortArr(startPort, endPort)
			for i := 0; i <= 50; i++ {
				go func(port int) {
					trueAddress := ip + ":" + strconv.Itoa(port)
					logs.Trace("try send test packet to target %s", trueAddress)
					remoteUdpAddr, err := net.ResolveUDPAddr("udp", trueAddress)
					if err != nil {
						return
					}
					ticker := time.NewTicker(time.Second * 2)
					defer ticker.Stop()
					for {
						select {
						case <-ticker.C:
							if isClose {
								return
							}
							if _, err := localConn.WriteTo([]byte(common.WORK_P2P_CONNECT), remoteUdpAddr); err != nil {
								return
							}
						}
					}
				}(ports[i])
				time.Sleep(time.Millisecond * 10)
			}
		}()

	}

	buf := make([]byte, 10)
	for {
		localConn.SetReadDeadline(time.Now().Add(time.Second * 10))
		n, addr, err := localConn.ReadFromUDP(buf)
		localConn.SetReadDeadline(time.Time{})
		if err != nil {
			break
		}
		switch string(buf[:n]) {
		case common.WORK_P2P_SUCCESS:
			for i := 20; i > 0; i-- {
				if _, err = localConn.WriteTo([]byte(common.WORK_P2P_END), addr); err != nil {
					return "", err
				}
			}
			return addr.String(), nil
		case common.WORK_P2P_END:
			logs.Debug("Remotely Address %v Reply Packet Successfully Received", addr)
			return addr.String(), nil
		case common.WORK_P2P_CONNECT:
			go func() {
				for i := 20; i > 0; i-- {
					logs.Debug("try send receive success packet to target %v", addr)
					if _, err = localConn.WriteTo([]byte(common.WORK_P2P_SUCCESS), addr); err != nil {
						return
					}
					time.Sleep(time.Second)
				}
			}()
		default:
			continue
		}
	}
	return "", errors.New("connect to the target failed, maybe the nat type is not support p2p")
}

func newUdpConnByAddr(addr string) (*net.UDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}
	return udpConn, nil
}

func getNextAddr(addr string, n int) (string, error) {
	lastColonIndex := strings.LastIndex(addr, ":")
	if lastColonIndex == -1 {
		return "", fmt.Errorf("the format of %s is incorrect", addr)
	}

	host := addr[:lastColonIndex]
	portStr := addr[lastColonIndex+1:]

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", err
	}

	return host + ":" + strconv.Itoa(port+n), nil
}

func getAddrInterval(addr1, addr2, addr3 string) (int, error) {
	p1 := common.GetPortByAddr(addr1)
	if p1 == 0 {
		return 0, fmt.Errorf("the format of %s incorrect", addr1)
	}
	p2 := common.GetPortByAddr(addr2)
	if p2 == 0 {
		return 0, fmt.Errorf("the format of %s incorrect", addr2)
	}
	p3 := common.GetPortByAddr(addr3)
	if p3 == 0 {
		return 0, fmt.Errorf("the format of %s incorrect", addr3)
	}
	interVal := int(math.Floor(math.Min(math.Abs(float64(p3-p2)), math.Abs(float64(p2-p1)))))
	if p3-p1 < 0 {
		return -interVal, nil
	}
	return interVal, nil
}

func getRandomPortArr(min, max int) []int {
	if min > max {
		min, max = max, min
	}
	length := max - min + 1
	addrAddr := make([]int, length)
	for i := 0; i < length; i++ {
		addrAddr[i] = max - i
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := length - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		addrAddr[i], addrAddr[j] = addrAddr[j], addrAddr[i]
	}
	return addrAddr
}
