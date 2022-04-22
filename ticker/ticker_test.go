package ticker

import (
	"fmt"
	"testing"
)

func TestStops(t *testing.T) {
	stop := make(chan bool, 1)
	close(stop)

	fmt.Println(<-stop)
	fmt.Println(<-stop)
}
