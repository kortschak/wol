// Copyright ©2020 Dan Kortschak. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// wake_server is an http endpoint to broadcast WOL packets.
//
// e.g. http://localhost:8080/<comma separated mac address list>?params
//
// Where valid params are:
//  via: specify the local address or device to send on
//  remote: specify the remote address to send to (default: 255.255.255.255:9)
//  pass: specify the wake password for all targets - 12 digit hex number
package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/kortschak/wol"
)

func main() {
	httpAddr := flag.String("http", "localhost:8080", "HTTP service address and port")
	flag.Parse()

	ln, err := net.Listen("tcp", *httpAddr)
	if err != nil {
		log.Fatal(err)
	}
	if !ln.Addr().(*net.TCPAddr).IP.IsLoopback() {
		log.Print("WARNING: listing on a non-loopback device")
	}
	ln.Close()

	http.HandleFunc("/", wakeHander)
	log.Fatal(http.ListenAndServe(*httpAddr, nil))
}

func wakeHander(w http.ResponseWriter, req *http.Request) {
	u, err := url.Parse(req.RequestURI)
	if err != nil {
		fmt.Printf("could not parse request: %v", err)
		return
	}

	macs := strings.Split(strings.TrimPrefix(u.Path, "/"), ",")
	if len(macs) == 0 {
		fmt.Fprintln(os.Stderr, "must specify at least one target MAC address")
		return
	}

	values := u.Query()
	via, err := parameter(values["via"])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid parameter: %v\n", err)
		return
	}
	local, err := route(via)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid parameter: %v\n", err)
		return
	}
	remote, err := parameter(values["remote"])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid parameter: %v\n", err)
		return
	}
	if remote == "" {
		remote = "255.255.255.255:9"
	}
	pass, err := parameter(values["pass"])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid parameter: %v\n", err)
		return
	}

	raddr, err := net.ResolveUDPAddr("udp", remote)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not parse remote %q as a valid UDP address: %v\n", remote, err)
		return
	}
	var laddr *net.UDPAddr
	if local != "" {
		laddr, err = net.ResolveUDPAddr("udp", local)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not parse local %q as a valid UDP address: %v\n", local, err)
			return
		}
	}

	var pw []byte
	switch len(pass) {
	case 0:
	case 12:
		pw, err = hex.DecodeString(pass)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse password: %v\n", err)
			return
		}
	default:
		fmt.Fprintf(os.Stderr, "invalid password: must be 12 hex digits long: %q\n", pass)
		return
	}

	for _, m := range macs {
		mac, err := net.ParseMAC(m)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not parse %q as a valid MAC address: %v\n", m, err)
			continue
		}
		err = wol.Wake(mac, pw, laddr, raddr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error attempting to wake %q: %v\n", mac, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "sent wake packet to %q\n", mac)
		fmt.Fprintf(w, "👋 %s\n", mac)
	}
}

func parameter(s []string) (string, error) {
	if len(s) == 0 {
		return "", nil
	}
	if len(s) > 1 {
		return "", fmt.Errorf("too many parameters: %v", s)
	}
	return s[0], nil
}

func route(dev string) (string, error) {
	host := dev
	var (
		port string
		err  error
	)
	if strings.Contains(host, ":") {
		host, port, err = net.SplitHostPort(host)
		if err != nil {
			return dev, err
		}
	}
	cmd := exec.Command("ip", "route", "show", "proto", "kernel", "dev", host)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	err = cmd.Run()
	if err != nil {
		if bytes.HasPrefix(buf.Bytes(), []byte("Cannot find device")) {
			return dev, nil
		}
		return "", fmt.Errorf("ip: %v", err)
	}

	host = buf.String()
	const srcSel = " src "
	idx := strings.Index(host, srcSel)
	if idx < 0 {
		return "", fmt.Errorf("no src selector: %q", host)
	}
	host = strings.SplitN(host[idx+len(srcSel):], " ", 2)[0]
	if port == "" {
		port = "0"
	}
	return host + ":" + port, nil
}
