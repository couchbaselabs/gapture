package examples

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

func ExprRecv() int {
	ch := make(chan int, 10)
	ch <- 1
	ch <- 2
	ch <- 3
	i := <-ch * (<-ch + 2)
	ch <- i
	fmt.Printf("%d", <-ch+100)
	return <-ch * <-ch
}

func SelectExample() int {
	ch := make(chan int, 10)
	x := 1

	select {
	case m := <-ch:
		x = m
	case m, ok := <-ch:
		if ok {
			x = m
		}
	case ch <- 2:
		x = 2
	default:
		x = 0
	}

	return x
}

func CloseExample() {
	ch := make(chan int, 10)
	if true {
		close(ch)
	}
}

func RangeExample(ch chan int) int {
	x := 1
	for m := range ch {
		x = m
		for range ch {
			x++
		}
	}
	return x
}
