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

// GID is a goroutine id.
type GID int64

// GCtx is a goroutine context.
type GCtx struct {
	GID    GID
	OpCtxs []OpCtx
}

// OpCtx associates an operation with context.
type OpCtx struct {
	Op     Op
	Stack  string
	Target interface{} // Depends on the operation; ex: a channel.
}

type Op int

const (
	OP_NONE Op = iota
	OP_CH_CLOSE
	OP_CH_SEND
	OP_CH_RECV
	OP_CH_SELECT_SEND
	OP_CH_SELECT_RECV
	OP_CH_RANGE
)

var OpStrings = map[Op]string{
	OP_NONE:           "none",
	OP_CH_CLOSE:       "ch-close",
	OP_CH_SEND:        "ch-send",
	OP_CH_RECV:        "ch-recv",
	OP_CH_SELECT_SEND: "ch-select-send",
	OP_CH_SELECT_RECV: "ch-select-recv",
	OP_CH_RANGE:       "ch-range",
}

// ---------------------------------------------------------------

var DefaultStackBufSize = 1000

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

// ---------------------------------------------------------------

// EnsureGID captures the current GID, if not already.
func (gctx *GCtx) EnsureGID() {
	if gctx.GID <= 0 {
		gctx.GID = CurrentGID()
	}
}

func (gctx *GCtx) AddOpCtx(op Op, target interface{}) interface{} {
	gctx.EnsureGID()
	gctx.OpCtxs = append(gctx.OpCtxs, OpCtx{
		Op:     op,
		Stack:  CurrentStack(2),
		Target: target,
	})
	return target
}

func (gctx *GCtx) ClearOpCtxs() {
	gctx.OpCtxs = nil
}

// ---------------------------------------------------------------

func (gctx *GCtx) OnChanClose(ch interface{}) interface{} {
	return gctx.AddOpCtx(OP_CH_CLOSE, ch)
}

func (gctx *GCtx) OnChanCloseDone() {
	gctx.ClearOpCtxs()
}

// ---------------------------------------------------------------

func (gctx *GCtx) OnChanSend(ch interface{}) interface{} {
	return gctx.AddOpCtx(OP_CH_SEND, ch)
}

func (gctx *GCtx) OnChanSendDone() {
	gctx.ClearOpCtxs()
}

// ---------------------------------------------------------------

func (gctx *GCtx) OnChanRecv(ch interface{}) interface{} {
	return gctx.AddOpCtx(OP_CH_RECV, ch)
}

func (gctx *GCtx) OnChanRecvDone(v interface{}) interface{} {
	gctx.ClearOpCtxs()
	return v
}

// ---------------------------------------------------------------

func (gctx *GCtx) OnChanSelectSend(caseNum int, ch interface{}) interface{} {
	if len(gctx.OpCtxs) > caseNum {
		panic("unexpected gapture.OnChanSelectSend caseNum")
	}
	return gctx.AddOpCtx(OP_CH_SELECT_SEND, ch)
}

func (gctx *GCtx) OnChanSelectSendDone(caseNum int) {
	gctx.ClearOpCtxs()
}

// ---------------------------------------------------------------

func (gctx *GCtx) OnChanSelectRecv(caseNum int, ch interface{}) interface{} {
	if len(gctx.OpCtxs) > caseNum {
		panic("unexpected gapture.OnChanSelectRecv caseNum")
	}
	return gctx.AddOpCtx(OP_CH_SELECT_RECV, ch)
}

func (gctx *GCtx) OnChanSelectRecvDone(caseNum int) {
	gctx.ClearOpCtxs()
}

// ---------------------------------------------------------------

func (gctx *GCtx) OnChanSelectDefault() {
	gctx.ClearOpCtxs()
}

// ---------------------------------------------------------------

func (gctx *GCtx) OnChanRange(ch interface{}) interface{} {
	return gctx.AddOpCtx(OP_CH_RANGE, ch)
}

func (gctx *GCtx) OnChanRangeBody() interface{} {
	if len(gctx.OpCtxs) != 1 ||
		gctx.OpCtxs[0].Op != OP_CH_RANGE {
		panic("unexpected gapture.OnChanRangeBody")
	}
	rv := gctx.OpCtxs[0].Target
	gctx.ClearOpCtxs()
	return rv
}

func (gctx *GCtx) OnChanRangeBodyContinue(ch interface{}) {
	gctx.AddOpCtx(OP_CH_RANGE, ch)
}

func (gctx *GCtx) OnChanRangeDone() {
	gctx.ClearOpCtxs()
}
