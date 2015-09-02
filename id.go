// Copyright 2015 Joshua Baker <joshbaker77@gmail.com>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package spin

import (
	"crypto/rand"
	"encoding/hex"
)

const idSize = 16

type Id [idSize]byte

func NewId() Id {
	var b = [idSize]byte{}
	rand.Read(b[:])
	return Id(b)
}

func IsValidIdString(s string) bool {
	if len(s) != idSize*2 {
		return false
	}
	for i := 0; i < idSize*2; i++ {
		switch s[i] {
		default:
			return false
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		case 'a', 'b', 'c', 'd', 'e', 'f':
		case 'A', 'B', 'C', 'D', 'E', 'F':
		}
	}
	return true
}

func IsValidIdBytes(b []byte) bool {
	return len(b) == idSize
}

func IdBytes(b []byte) Id {
	var id Id
	copy(id[:], b)
	return id
}

func IdString(s string) Id {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return IdBytes(b)
}

func (id Id) Bytes() []byte {
	return id[:]
}

func (id Id) String() string {
	return hex.EncodeToString(id.Bytes())
}
