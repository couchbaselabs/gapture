//  Copyright (c) 2016 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the
//  License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing,
//  software distributed under the License is distributed on an "AS
//  IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
//  express or implied. See the License for the specific language
//  governing permissions and limitations under the License.

package gapture

import (
	"bytes"
	"runtime"
	"strconv"
)

// A goroutine id.
type GID int64

var DefaultStackBufSize = 4000

var ExpectedStackPrefix = []byte("goroutine ")
var ExpectedStackPrefixLen = len(ExpectedStackPrefix)

// Stack returns the goroutine id and stack frames.
func Stack(skipFrames int) (GID, string) {
	buf := make([]byte, DefaultStackBufSize)

	n := runtime.Stack(buf, false)

	if !bytes.HasPrefix(buf, ExpectedStackPrefix) {
		panic("unexpected stack prefix")
	}
	buf = buf[ExpectedStackPrefixLen:n]

	gidBuf := buf[0:bytes.IndexByte(buf, ' ')] // The goroutine id.
	gid, err := strconv.ParseInt(string(gidBuf), 10, 64)
	if err != nil {
		panic(err)
	}
	buf = buf[len(gidBuf)+1:]

	stackBuf := buf[bytes.IndexByte(buf, '\n')+1:]
	for i := 0; i <= skipFrames; i++ { // Always skip at least 1 frame.
		stackBuf = stackBuf[bytes.IndexByte(stackBuf, '\n')+1:]
		stackBuf = stackBuf[bytes.IndexByte(stackBuf, '\n')+1:]
	}

	return GID(gid), string(stackBuf)
}

// Return string of Stack() looks like the following (and has "\t" tabs)...
//
// goroutine 1 [running]:
// main.foo(0x0, 0x0)
//     /Users/steveyen/go/src/github.com/couchbaselabs/gapture/main.go:128 +0x7b
// main.main()
//     /Users/steveyen/go/src/github.com/couchbaselabs/gapture/main.go:27 +0x27
