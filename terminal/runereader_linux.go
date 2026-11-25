// copied from https://github.com/golang/crypto/blob/master/ssh/terminal/util_linux.go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//go:build linux
// +build linux

package terminal

import "golang.org/x/sys/unix"

// The request numbers come from x/sys/unix, which declares the right value per
// architecture instead of relying on one hand-copied pair per file. That also
// fixes linux/mips and linux/ppc64, whose TCGETS is not 0x5401.
const (
	ioctlReadTermios  = unix.TCGETS
	ioctlWriteTermios = unix.TCSETS
)
