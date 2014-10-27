receives

  recvExpr[m], recvOk[ok] := <-expr(whatever)

    gapture_ch_sym_001 := expr(whatever)
    BEFORE_RECV(gapture_ch_sym_001)
    gapture_msg_sym_002, gapture_ok_sym_003 := <-gapture_ch_sym_001
    AFTER_RECV(gapture_ch_sym_001, gapture_msg_sym_002, gapture_ok_sym_003)
    recvExpr[m], recvOk[ok] := gapture_msg_sym_002, gapture_ok_sym_003

sends

  chanExpr(x) <- msgExpr(m)

    gapture_msg_sym_001 := msgExpr(m)
    gapture_ch_sym_002 := chanExpr(m)
    BEFORE_SEND(gapture_ch_sym_002, gapture_msg_sym_001)
    gapture_ch_sym_002 <- gapture_msg_sym_001
    AFTER_SEND(gapture_ch_sym_002)

ranges

  for m := range someChan(c) {
    ...block...
  }

  for k, v := range someMap(m) {
    ...block...
  }

  for _, v := range someArray(a) {
    ...block...
  }

    gapture_ch_sym_001 := someChan(c)
    BEFORE_RANGE_RECV(gapture_ch_sym_001)
    for gapture_msg_sym_002 := range gapture_ch_sym_001 {
       AFTER_RANGE_RECV(gapture_ch_sym_001, gapture_msg_sym_002, true)
       m = gapture_msg_sym_002
       ...block...
       if (foo) {
         BEFORE_RANGE_RE_RECV(gapture_ch_sym_001)
         continue
       }
       BEFORE_RANGE_RE_RECV(gapture_ch_sym_001)
    }
    AFTER_RANGE_RECV(gapture_ch_sym_001, nil, false)

selects

  select {
  case m := <-someChan(c):
     ...block...
  case someChan2(c) <- msgExpr(m):
     ...block...
  default:
     ...block...
  }

    gapture_ch_sym_001 := someChan(c)
    gapture_ch_sym_001 := someChan2(c)
    gapture_msg_sym_002 := msgExpr(m)
    BEFORE_SELECT(gapture_ch_sym_001, gapture_msg_sym_002)
    select {
    case m := <-exprChan(c):
       AFTER_SELECT(...)
       ...block...
    case c <- exprMsg(m):
       AFTER_SELECT(...)
       ...block...
    default:
       AFTER_SELECT(...)
       ...block...
    }
    AFTER_SELECT(...)

go

  go foo(1, 2)
  go mgr.foo(1, 2)

close

  close(exprCh(c))
