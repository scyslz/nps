//go:build !windows
// +build !windows

package proxy

import (
	"fmt"
	"net"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/djylb/nps/lib/common"
	"github.com/djylb/nps/lib/conn"
	"github.com/djylb/nps/lib/file"
	"golang.org/x/sys/unix"
)

func HandleTrans(c *conn.Conn, s *TunnelModeServer) error {
	if addr, err := getAddress(c.Conn); err != nil {
		return err
	} else {
		return s.DealClient(c, s.task.Client, addr, nil, common.CONN_TCP, nil, []*file.Flow{s.task.Flow, s.task.Client.Flow}, s.task.Target.ProxyProtocol, s.task.Target.LocalProxy, s.task)
	}
}

const SO_ORIGINAL_DST = 80

func getAddress(conn net.Conn) (string, error) {
	sysconn, ok := conn.(syscall.Conn)
	if !ok {
		return "", fmt.Errorf("connection does not support SyscallConn")
	}
	raw, err := sysconn.SyscallConn()
	if err != nil {
		return "", err
	}

	var dst string
	var opErr error

	err = raw.Control(func(fd uintptr) {
		var sa unix.RawSockaddrAny
		sz := uint32(unsafe.Sizeof(sa))
		_, _, errno := unix.Syscall6(
			unix.SYS_GETSOCKOPT,
			fd,
			uintptr(unix.SOL_SOCKET),
			uintptr(SO_ORIGINAL_DST),
			uintptr(unsafe.Pointer(&sa)),
			uintptr(unsafe.Pointer(&sz)),
			0,
		)
		if errno == 0 {
			switch sa.Addr.Family {
			case unix.AF_INET:
				// IPv4
				sa4 := (*unix.RawSockaddrInet4)(unsafe.Pointer(&sa))
				ip := net.IPv4(sa4.Addr[0], sa4.Addr[1], sa4.Addr[2], sa4.Addr[3])
				port := int(sa4.Port>>8)&0xff | int(sa4.Port&0xff)<<8
				dst = ip.String() + ":" + strconv.Itoa(port)
			case unix.AF_INET6:
				// IPv6
				sa6 := (*unix.RawSockaddrInet6)(unsafe.Pointer(&sa))
				ip := net.IP(sa6.Addr[:])
				port := int(sa6.Port>>8)&0xff | int(sa6.Port&0xff)<<8
				dst = "[" + ip.String() + "]:" + strconv.Itoa(port)
			default:
				opErr = fmt.Errorf("unsupported address family: %d", sa.Addr.Family)
			}
			return
		}

		opErr = fmt.Errorf("not a redirected connection (errno=%v)", errno)
	})

	if err != nil {
		return "", err
	}
	if opErr != nil {
		return "", opErr
	}
	return dst, nil
}
