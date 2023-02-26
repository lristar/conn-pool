package pool

import "time"

type idleConn struct {
	conn       IConn
	t          time.Time
	isDuration bool
}

func newIdleConn(conn IConn, isDuration bool) *idleConn {
	return &idleConn{
		conn:       conn,
		t:          time.Now(),
		isDuration: isDuration,
	}
}

// Duration 持久化
func (i *idleConn) duration() *idleConn {
	i.isDuration = true
	return i
}

func (i *idleConn) timeOut(delayTime time.Duration) (to bool) {
	if !i.isDuration {
		if i.t.Add(delayTime).Before(time.Now()) {
			to = true
		}
	}
	return
}

func (i *idleConn) renew() {
	i.t = time.Now()
}

func (i *idleConn) Handle(h func(conn IConn) error) error {
	defer i.renew()
	return h(i.conn)
}
