package fitness

import "fmt"

func ExampleLogDnorm() {
	fmt.Println(LogDnorm(1.0, 0.0, 1.0))

	// Output:
	// -1.4189385332046727
}

func ExampleDnorm() {
	fmt.Println(Dnorm(1.0, 0.0, 1.0))

	// Output:
	// 0.24197072451914337
}
