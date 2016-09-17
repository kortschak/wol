// Copyright ©2016 Dan Kortschak. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// wake demonstrates the use of the wol.Wake function.
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/kortschak/wol"
)

func main() {
	local := flag.String("via", "", "specify the local address to send on")
	remote := flag.String("remote", "255.255.255.255:9", "specify the remote address to send to")
	pass := flag.String("pass", "", "specify the wake password for all targets - 12 digit hex number")
	flag.Parse()
	if len(flag.Args()) == 0 {
		fmt.Fprintln(os.Stderr, "must specify at least one target MAC address")
		flag.Usage()
		os.Exit(1)
	}

	raddr, err := net.ResolveUDPAddr("udp", *remote)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not parse remote %q as a valid UDP address: %v\n", *remote, err)
		os.Exit(1)
	}
	var laddr *net.UDPAddr
	if *local != "" {
		laddr, err = net.ResolveUDPAddr("udp", *local)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not parse local %q as a valid UDP address: %v\n", *local, err)
			os.Exit(1)
		}
	}

	var pw []byte
	switch len(*pass) {
	case 0:
	case 12:
		pw, err = hex.DecodeString(*pass)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse password: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "invalid password: must be 12 hex digits long: %q\n", *pass)
		os.Exit(1)
	}

	for _, m := range flag.Args() {
		mac, err := net.ParseMAC(m)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not parse %q as a valid MAC address: %v\n", m, err)
		}
		err = wol.Wake(mac, pw, laddr, raddr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error attempting to wake %s: %v\n", mac, err)
		}
	}
}
