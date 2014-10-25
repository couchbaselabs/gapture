package main

import "fmt"
import "runtime"

func gostack(c chan bool) {
	buf := make([]byte, 6000000)
	n := runtime.Stack(buf, false)

	fmt.Println("current:", string(buf[0:n]))
	if c != nil {
		c <- true
	}
}

func main() {
	gostack(nil)
	gostack(nil)
	f()
	gostack(nil)
}

func f() {
	c := make(chan bool)
	go gostack(c)
	gostack(nil)
	<-c
	go gostack(c)
	gostack(nil)
	<-c
	gostack(nil)
}
