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

// Package gapture provides runtime facilities for goroutine behavior
// capture and playback.  e.g., "go-capture" ==> gapture.
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

// CurrentGID returns the goroutine id.
func CurrentGID() GID {
	buf := make([]byte, 64)
	n := runtime.Stack(buf, false)
	buf = buf[0:n]

	if !bytes.HasPrefix(buf, ExpectedStackPrefix) {
		panic("unexpected stack prefix")
	}
	buf = buf[ExpectedStackPrefixLen:n]

	gidBuf := buf[0:bytes.IndexByte(buf, ' ')]
	gid, err := strconv.ParseInt(string(gidBuf), 10, 64)
	if err != nil {
		panic(err)
	}

	return GID(gid)
}

// CurrentStack returns the call stack.  The returned
// stack string looks like the following (and has "\t" tabs)...
//
// github.com/couchbaselabs/gapture.ExampleStack()
// 	/Users/steveyen/go/src/github.com/couchbaselabs/gapture/gapture.go:76 +0x3a
// main.main()
// 	/Users/steveyen/go/src/github.com/couchbaselabs/gapture/cmd/gapture/main.go:32 +0x195
//
func CurrentStack(skipFrames int) string {
	buf := make([]byte, DefaultStackBufSize)
	n := runtime.Stack(buf, false)
	buf = buf[0:n]
	buf = buf[bytes.IndexByte(buf, '\n')+1:] // Skip first goroutine line.
	for i := 0; i <= skipFrames; i++ {       // Always skip 1 frame for CurrentStack().
		buf = buf[bytes.IndexByte(buf, '\n')+1:]
		buf = buf[bytes.IndexByte(buf, '\n')+1:]
	}
	return string(buf)
}
