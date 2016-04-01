go GC trace data viewer
- https://github.com/davecheney/gcvis

go AST viewer
- https://github.com/yuroyoro/goast-viewer

go error checker - checks that you checked errors
- https://github.com/kisielk/errcheck

go oracle tool
- https://godoc.org/golang.org/x/tools/oracle

recv expressions are tough
 whatever || <-c || <-d || whatever

------------------------------------------------------------
we can know that a goroutine is sending/receiving/selecting.
we can know the time.
we can know the channel (its len() and cap()).

but, having a global map could be slow.
a sharded map might help.

------------------------------------------------------------
Statement/expression conversions:

  ------------------------------------------
  Convert:
	close(chExpr)
  Into:
	close(gaptureGCtx.OnChanClose(chExpr).(chan foo))
	gaptureGCtx.OnChanCloseDone()

  ------------------------------------------
  Convert:
    chExpr <- msgExpr
  Into:
    gaptureGCtx.OnChanSend(chExpr).(chan foo) <- msgExpr
    gaptureGCtx.OnChanSendDone()

  ------------------------------------------
  Convert:
    <-chExpr
  Into:
    <-gaptureCtx.OnChanRecv(chExpr).(chan foo)
    gaptureCtx.OnChanRecvDone()

  NOTE: We don't handle general recv expressions (ex: <-ch1 && <-ch2).

  ------------------------------------------
  Convert:
    select {
    case msg := <-recvCh:
    case sendCh <- msgExpr:
    default:
    }
  Into:
    select {
    case msg := <-gaptureCtx.OnSelectChanRecv(0, recvCh).(chan foo):
      gaptureCtx.OnSelectChanRecvDone(0)
    case gaptureGCtx.OnSelectChanSend(1, chExpr).(chan foo) <- msgExpr:
      gaptureGCtx.OnSelectChanSendDone(1)
    default:
      gaptureCtx.OnSelectDefault()
    }

  ------------------------------------------
  for range chExpr { ... }   y   y   y       y

    for range gaptureCtx.OnChan(gapture.CHAN_RANGE, chExpr) {
      gaptureCtx.OnDone(gapture.CHAN_RANGE_WAIT, -1)
      ...
         // ISSUE: any continue's in here would skip the gapture.OnChan!!!
      ...
      gaptureCtx.OnChan(gapture.CHAN_RANGE_WAIT, nil)
    }
    gaptureCtx.OnDone(gapture.CHAN_RANGE, 0)

  ------------------------------------------
  cgo call
    TODO.

  ------------------------------------------
  panic(...)
    NOT CONVERTED.

  ------------------------------------------
  go funcExpr(...)
    NOT CONVERTED.
