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
Statement/expression:        Does it need markup/rewrite?
                             beg end bodyBeg bodyEnd
  go funcExpr(...)           n   n

  ------------------------------------------
  close(chExpr)              y   y

    close(gaptureCtx.EventChan(gapture.CHAN_CLOSE, chExpr))
    gaptureCtx.EventDone(gapture.CHAN_CLOSE, 0)

  ------------------------------------------
  chExpr <- msgExpr          y   y

    gaptureCtx.EventChan(gapture.CHAN_SEND, chExpr) <- msgExpr
    gaptureCtx.EventDone(gapture.CHAN_SEND, 0)

  ------------------------------------------
  <-chExpr                   y   y

    <-gaptureCtx.EventChan(gapture.CHAN_RECV, chExpr)
    gaptureCtx.EventDone(gapture.CHAN_RECV, 0)

  ------------------------------------------
  for range chExpr { ... }   y   y   y       y

    for range gaptureCtx.EventChan(gapture.CHAN_RANGE, chExpr) {
      gaptureCtx.EventDone(gapture.CHAN_RANGE_WAIT, -1)
      ...
         // ISSUE: any continue's in here would skip the gapture.EventBeg!!!
      ...
      gaptureCtx.EventChan(gapture.CHAN_RANGE_WAIT, nil)
    }
    gaptureCtx.EventDone(gapture.CHAN_RANGE, 0)

  ------------------------------------------
  select {                   y   y+ (every caseStmt and default)
    case msg := <-recvCh:
    case sendCh <- msg:
    default:
  }

    select {
      case msg := <-gaptureCtx.EventChan(gapture.CHAN_SELECT_RECV, recvCh):
        gaptureCtx.EventDone(gapture.CHAN_SELECT, 0)

      case gaptureCtx.EventChan(gapture.CHAN_SELECT_SEND, sendCh) <- msg:
        gaptureCtx.EventDone(gapture.CHAN_SELECT, 1)

      default:
        gaptureCtx.EventDone(gapture.CHAN_SELECT, -1)
    }

  ------------------------------------------
  cgo call                   y   y

  ------------------------------------------
  panic(...)                 n   n
