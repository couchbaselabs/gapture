package main

// A simple example.

func main() {
	ch := make(chan int)

	go func() {
		// A child goroutine.
		ch <- 42

		close(ch)
	}()

	<-ch
}
