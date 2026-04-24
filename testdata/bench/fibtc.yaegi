package main

import "fmt"

func fib(x, a, b int) int {
	if x == 0 {
		return a
	}
	if x == 1 {
		return b
	}
	return fib(x-1, b, a+b)
}

func main() {
	fmt.Println(fib(35, 0, 1))
}
