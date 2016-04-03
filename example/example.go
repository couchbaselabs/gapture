package example

import (
	"fmt"
)

type Msg struct {
	From    string
	Body    map[string]string
	ReplyCh chan Msg
}

func Pinger(name string, ch chan Msg, n int, sync bool) {
	for i := 0; i < n; i++ {
		m := Msg{From: name, Body: map[string]string{
			"i": fmt.Sprintf("%d", i),
			"n": fmt.Sprintf("%d", n),
		}}
		if sync {
			m.ReplyCh = make(chan Msg)
		}
		ch <- m
		if m.ReplyCh != nil {
			<-m.ReplyCh
		}
	}
}

func Ponger(name string, ch chan Msg) {
	i := 0
	for {
		msg, ok := <-ch
		if !ok {
			return
		}
		if msg.ReplyCh != nil {
			close(msg.ReplyCh)
		}
		i++
	}
}
