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
	ch := make(chan bool)
	close(ch)

	rv := false
	for t := range ch {
		rv = rv || t
	}
}