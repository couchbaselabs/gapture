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

  close(chExpr)              y   y

  chExpr <- msgExpr          y   y

  <-chExpr                   y   y

  for range chExpr { ... }   y   y   y       y

  select {                   y   y+ (every caseStmt and default)
     case sendOrRecvExpr:
     default:
  }

  cgo call                   y   y

  panic(...)                 n   n
