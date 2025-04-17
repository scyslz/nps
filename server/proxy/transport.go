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
		// IPv4
		var sa4 syscall.RawSockaddrInet4
		sz4 := uint32(unsafe.Sizeof(sa4))
		_, _, errno4 := syscall.Syscall6(
			syscall.SYS_GETSOCKOPT,
			fd,
			uintptr(syscall.SOL_IP),
			uintptr(SO_ORIGINAL_DST),
			uintptr(unsafe.Pointer(&sa4)),
			uintptr(unsafe.Pointer(&sz4)),
			0,
		)
		if errno4 == 0 {
			ip := net.IPv4(sa4.Addr[0], sa4.Addr[1], sa4.Addr[2], sa4.Addr[3])
			port := int(sa4.Port>>8)&0xff | int(sa4.Port&0xff)<<8
			dst = ip.String() + ":" + strconv.Itoa(port)
			return
		}

		// IPv6
		var sa6 syscall.RawSockaddrInet6
		sz6 := uint32(unsafe.Sizeof(sa6))
		_, _, errno6 := syscall.Syscall6(
			syscall.SYS_GETSOCKOPT,
			fd,
			uintptr(syscall.SOL_IPV6),
			uintptr(SO_ORIGINAL_DST),
			uintptr(unsafe.Pointer(&sa6)),
			uintptr(unsafe.Pointer(&sz6)),
			0,
		)
		if errno6 == 0 {
			ip := net.IP(sa6.Addr[:])
			port := int(sa6.Port>>8)&0xff | int(sa6.Port&0xff)<<8
			dst = "[" + ip.String() + "]:" + strconv.Itoa(port)
			return
		}

		opErr = fmt.Errorf("not a redirected connection (errno4=%v, errno6=%v)", errno4, errno6)
	})

	if err != nil {
		return "", err
	}
	if opErr != nil {
		return "", opErr
	}
	return dst, nil
}
