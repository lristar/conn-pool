package pool

import (
	"fmt"
	"testing"
	"time"
)

var (
	num int32
)

func Test_idle(t *testing.T) {
	id := newIdleConn(new(Conn), false)
	fmt.Println(id.t)
	time.Sleep(time.Second * 5)
	id.renew()
	fmt.Println(id.t)
}
