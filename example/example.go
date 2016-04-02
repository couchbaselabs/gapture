package gapture

import (
	"fmt"
)

func Example() {
	fmt.Printf("an example function, useful to test convert processing")

	f := func(x int) int { return x + 1 }

	f(1)
}

func ExampleWithChan() {
	ch := make(chan bool, 1)
	ch <- true
	close(ch)
	b := <-ch

	func() {
		rv := false
		for t := range ch {
			rv = rv || t || b
		}

		select {
		case msg := <-ch:
			b = msg
		case ch <- false:
			b = false
		default:
			b = true
		}

		for msg := range ch {
			b = msg
		}
	}()

	var z interface{}
	z = ch
	ch2 := z.(chan bool)
	if ch2 == ch {
		fmt.Printf("yay")
	}
}