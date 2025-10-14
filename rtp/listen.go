// Copyright 2023 LiveKit, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rtp

import (
	"errors"
	"math/rand"
	"net"
	"net/netip"
)

var ErrListenFailed = errors.New("failed to listen on udp port")

func ListenUDPPortRange(portMin, portMax int, ip netip.Addr) (*net.UDPConn, error) {
	if portMin == 0 && portMax == 0 {
		return net.ListenUDP("udp", &net.UDPAddr{
			IP:   ip.AsSlice(),
			Port: 0,
		})
	}

	i := portMin
	if i == 0 {
		i = 1
	}

	j := portMax
	if j == 0 {
		j = 0xFFFF
	}

	if i > j {
		return nil, ErrListenFailed
	}

	portStart := rand.Intn(portMax-portMin+1) + portMin
	portCurrent := portStart

	for {
		c, e := net.ListenUDP("udp", &net.UDPAddr{IP: ip.AsSlice(), Port: portCurrent})
		if e == nil {
			return c, nil
		}

		portCurrent++
		if portCurrent > j {
			portCurrent = i
		}
		if portCurrent == portStart {
			break
		}
	}
	return nil, ErrListenFailed
}

func ReserveUDP() (conn *net.UDPConn, port int, fd uintptr, err error) {
	addr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
	conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return nil, 0, 0, err
	}
	local := conn.LocalAddr().(*net.UDPAddr)
	port = local.Port

	// Get a dup()'d file descriptor that GStreamer can own.
	// On Unix, UDPConn.File() duplicates the underlying FD.
	// NOTE: the dup is set to blocking; GStreamer expects/blocking is fine.
	f, err := conn.File()
	if err != nil {
		conn.Close()
		return nil, 0, 0, err
	}
	// We keep both the conn and the dup()'d FD alive. GStreamer will close its copy on finalize.
	fd = f.Fd()
	// Avoid leaking the *os.File wrapper; GStreamer holds the FD value.
	f.Close()
	return conn, port, fd, nil
}

func FindFreeUDPPort(ip netip.Addr) (port int, err error) {
	addr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return 0, err
	}
	local := conn.LocalAddr().(*net.UDPAddr)
	port = local.Port
	conn.Close()
	return port, nil
}
