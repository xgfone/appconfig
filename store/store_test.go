package store

import "fmt"

func ExampleGetStringPage() {
	data := []string{"a", "b", "c", "d", "e", "f", "g"}
	fmt.Println(GetStringPage(data, int64(2), int64(3)))

	// Output:
	// [d e f]
}
