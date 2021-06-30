package UnitTest

import (
	"fmt"
	"sync"
	"testing"
)

var wait sync.WaitGroup

func TestLock(t *testing.T) {
	wait.Add(1)
	wait.Wait()
	fmt.Println("freedom")

}

func TestUnLock(t *testing.T) {
	wait.Done()
}
