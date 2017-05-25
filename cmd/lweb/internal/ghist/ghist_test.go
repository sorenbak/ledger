package ghist

import (
	"fmt"
	"testing"
)

func TestGet(t *testing.T) {
	prices, err := Get("IVV")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(prices)
}
