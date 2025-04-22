package common

import (
	"context"
	"fmt"
	"github.com/xtaci/kcp-go/v5"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

var customDnsAddr string

func SetCustomDNS(dnsAddr string) {
	if dnsAddr == "" {
		return
	}
	colonCount := strings.Count(dnsAddr, ":")
	if colonCount == 0 {
		dnsAddr += ":53"
	} else if colonCount > 1 && !strings.Contains(dnsAddr, "]:") {
		if strings.Contains(dnsAddr, "]") {
			dnsAddr += ":53"
		} else {
			dnsAddr = "[" + dnsAddr + "]:53"
		}
	}

	customDnsAddr = dnsAddr

	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return net.Dial(network, dnsAddr)
		},
	}
}

func GetCustomDNS() string {
	if customDnsAddr != "" {
		return customDnsAddr
	}
	return "8.8.8.8:53"
}

func GetFastAddr(addr string, testType string) (string, error) {
	dnsServer := GetCustomDNS()
	host := GetIpByAddr(addr)

	ip := net.ParseIP(host)
	if ip != nil {
		return addr, nil
	}

	port := GetPortByAddr(addr)

	var ipv4List, ipv6List []string
	ipv4List, ipv6List, err := resolveDomain(host, ipv4List, ipv6List, dnsServer, 10)

	if err != nil || (len(ipv4List) == 0 && len(ipv6List) == 0) {
		return addr, fmt.Errorf("Can not resolve %s", host)
	}

	ipList := append(ipv4List, ipv6List...)

	bestIP, bestLatency := ipList[0], time.Duration(1<<63-1)

	for _, ip := range ipList {
		latency, err := TestLatency(BuildAddress(ip, strconv.Itoa(port)), testType)
		if err != nil {
			continue
		}
		if latency < bestLatency {
			bestIP, bestLatency = ip, latency
		}
	}
	return BuildAddress(bestIP, strconv.Itoa(port)), nil
}

func resolveDomain(domain string, ipv4List, ipv6List []string, dnsServer string, redirects int) ([]string, []string, error) {
	if redirects <= 0 {
		return ipv4List, ipv6List, fmt.Errorf("Too many CNAME")
	}

	ipv4s, ipv6s, err := resolveIPs(domain, dnsServer)
	if err != nil {
		return ipv4List, ipv6List, err
	}

	ipv4List = append(ipv4List, ipv4s...)
	ipv6List = append(ipv6List, ipv6s...)

	cnameList, err := resolveDNS(domain, dns.TypeCNAME, dnsServer)
	if err != nil || len(cnameList) == 0 {
		return ipv4List, ipv6List, nil
	}

	for _, cname := range cnameList {
		ipv4List, ipv6List, err = resolveDomain(cname, ipv4List, ipv6List, dnsServer, redirects-1)
		if err != nil {
			continue
		}
	}

	return ipv4List, ipv6List, nil
}

func resolveIPs(domain, dnsServer string) ([]string, []string, error) {
	ipv4List, err := resolveDNS(domain, dns.TypeA, dnsServer)
	if err != nil {
		return nil, nil, err
	}

	ipv6List, err := resolveDNS(domain, dns.TypeAAAA, dnsServer)
	if err != nil {
		return nil, nil, err
	}

	return ipv4List, ipv6List, nil
}

func resolveDNS(domain string, qtype uint16, dnsServer string) ([]string, error) {
	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), qtype)
	m.RecursionDesired = false

	resp, _, err := c.Exchange(m, dnsServer)
	if err != nil {
		return nil, err
	}

	var ipList []string
	for _, ans := range resp.Answer {
		switch r := ans.(type) {
		case *dns.A:
			ipList = append(ipList, r.A.String())
		case *dns.AAAA:
			ipList = append(ipList, r.AAAA.String())
		case *dns.CNAME:
			ipList = append(ipList, r.Target+r.String())
		}
	}
	return ipList, nil
}

func TestLatency(addr string, testType string) (time.Duration, error) {
	start := time.Now()
	switch testType {
	case "tcp", "tls":
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err != nil {
			return 0, err
		}
		defer conn.Close()
	case "kcp", "udp":
		conn, err := kcp.DialWithOptions(addr, nil, 0, 0)
		if err != nil {
			return 0, err
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(2 * time.Second))
	default:
		return 0, fmt.Errorf("Unsupported test type: %s", testType)
	}
	return time.Since(start), nil
}
