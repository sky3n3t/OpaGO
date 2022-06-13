package main

func main() {
}

//export factorial
func factorial(x int) int {
	/*var out int = 1
	for ; x > 0; x-- {
		out *= x
	}*/
	return x * x
}
