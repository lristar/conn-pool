package pool

import (
	"github.com/stretchr/testify/assert"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type Conn struct {
}

// Ping 检查连接是否有效的方法
func (c *Conn) Ping() error {
	return nil
}

func (c *Conn) Close() error {
	return nil
}

func (c *Conn) Use(v interface{}) error {
	time.Sleep(time.Millisecond * 50)
	logger.Infof("Use 调用了这个连接")
	return nil
}

func factory() (IConn, error) {
	return new(Conn), nil
}

func TestInitPool(t *testing.T) {
	aet := assert.New(t)
	pool, err := InitPool(SetFactory(factory))
	if err != nil {
		t.Fatal(err)
	}
	aet.Equal(int32(5), pool.InitialCap, "连接池初始化最小容量有误")
	aet.Equal(int32(20), pool.maxActive, "连接池初始化最大容量有误")
	aet.Equal(time.Second*20, pool.idleTimeout, "过期时间未达到期望")
	aet.Equal(int32(5), pool.openingConns, "已存在的连接与设定的初始值不匹配")
	aet.Equal(pool.openingConns, int32(len(pool.conns)), "管道内的连接数量与openingConns数值不匹配")

	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
}

// 初始化五个初始值,取五次后第六次报错
func TestGetConnNoWait(t *testing.T) {
	aet := assert.New(t)
	pool, err := InitPool(SetFactory(factory), SetIdleTimeout(time.Second*15))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		_, err = pool.getConnNoWait()
		if err != nil {
			aet.Equal(i, 5)
			aet.Equal("未获取到旧连接", err.Error())
			return
		}
	}
}

// put
func TestPut(t *testing.T) {
	aet := assert.New(t)
	pool, err := InitPool(SetFactory(factory), SetIdleTimeout(time.Second*15))
	if err != nil {
		aet.Error(err)
		return
	}
	id := make([]*idleConn, 0)
	for i := 0; i < 5; i++ {
		conn, err := pool.getConnNoWait()
		if err != nil {
			return
		}
		id = append(id, conn)
	}
	aet.Equal(0, len(pool.conns), "所有取出应该为空")
	for i := range id {
		pool.put(id[i])
		aet.Equal(i+1, len(pool.conns), "管道对应数值不对")
	}
}

// closeOne
func TestCloseOne(t *testing.T) {
	aet := assert.New(t)
	pool, err := InitPool(SetFactory(factory), SetIdleTimeout(time.Second*15))
	if err != nil {
		aet.Error(err)
		return
	}
	conn, err := pool.getConnNoWait()
	if err != nil {
		aet.Error(err)
		return
	}
	aet.Equal(4, len(pool.conns), "管道内是否正确")
	aet.Equal(5, int(pool.openingConns), "连接数值是否正确")
	pool.closeOne(conn)
	aet.Equal(4, int(pool.openingConns), "连接数值是否正确")
}

func TestChannelPool_Close(t *testing.T) {
	aet := assert.New(t)
	pool, err := InitPool(SetFactory(factory), SetIdleTimeout(time.Second*15))
	if err != nil {
		aet.Error(err)
		return
	}
	pool.Close()
	aet.Equal(0, len(pool.conns), "管道内是否正确")
	aet.Equal(0, int(pool.openingConns), "连接数值是否正确")
	// 这时候再进行使用handle是否有panic
	if err := pool.Handle(func(conn IConn) error {
		atomic.AddInt32(&num, 1)
		return conn.Use("hahahah")
	}); err != nil {
		aet.Error(err)
		return
	}
}

func TestChannelPool_Release(t *testing.T) {
	aet := assert.New(t)
	pool, err := InitPool(SetFactory(factory), SetIdleTimeout(time.Second*15))
	if err != nil {
		aet.Error(err)
		return
	}
	pool.Release()
	aet.Equal(0, len(pool.conns), "管道内是否正确")
	aet.Equal(0, int(pool.openingConns), "连接数值是否正确")
	// 这时候再进行使用handle是否有panic
	if err := pool.Handle(func(conn IConn) error {
		atomic.AddInt32(&num, 1)
		return conn.Use("hahahah")
	}); err != nil {
		aet.Error(err)
		return
	}
	aet.Equal(1, len(pool.conns), "管道内是否正确")
	aet.Equal(1, int(pool.openingConns), "连接数值是否正确")
}

func TestGetConnWait(t *testing.T) {
	aet := assert.New(t)
	pool, err := InitPool(SetFactory(factory), SetIdleTimeout(time.Second*15))
	if err != nil {
		aet.Error(err)
		return
	}
	num = 0
	w := sync.WaitGroup{}
	w.Add(20)
	for i := 0; i < 21; i++ {
		go func() {
			if err := pool.Handle(func(conn IConn) error {
				time.Sleep(time.Second * 11)
				atomic.AddInt32(&num, 1)
				defer w.Done()
				return conn.Use("hahah")
			}); err != nil {
				aet.Error(err)
			}
		}()
	}
	w.Wait()
	aet.Equal(20, int(num), "执行数量不等")
}

// 测试最大限制是否达到
func TestMaxnum(t *testing.T) {
	aet := assert.New(t)
	pool, err := InitPool(SetFactory(factory), SetIdleTimeout(time.Second*15))
	if err != nil {
		aet.Error(err)
		return
	}
	w := sync.WaitGroup{}
	num = 0
	for i := 0; i < 1000; i++ {
		w.Add(1)
		go func() {
			if err := pool.Handle(func(conn IConn) error {
				atomic.AddInt32(&num, 1)
				defer w.Done()
				return conn.Use("hahahah")
			}); err != nil {
				aet.Error(err)
				return
			}
		}()
	}
	w.Wait()
	// 测试数据
	aet.Equal(int32(1000), num, "处理数量不等")
	aet.Equal(20, len(pool.conns), "当前管道数值未达到期望的20")
	aet.Equal(pool.openingConns, int32(len(pool.conns)), "期望值不匹配")

	// 等待15秒后清楚临时连接只剩最小值
	time.Sleep(time.Second * 15)
	go func() {
		for i := 0; i < 6; i++ {
			w.Add(1)
			if err := pool.Handle(func(conn IConn) error {
				defer w.Done()
				return conn.Use("hahahah")
			}); err != nil {
				aet.Error(err)
				return
			}
		}
	}()
	w.Wait()
	time.Sleep(time.Second * 1)
	aet.Equal(5, len(pool.conns), "管道数值未达到最小值,过期的并未清理")
	aet.Equal(int32(len(pool.conns)), pool.openingConns, "管道内数值与记录数值不匹配")
	if err := pool.Close(); err != nil {
		aet.Error(err)
		return
	}
}

func TestChannelPool_GetActive(t *testing.T) {
	aet := assert.New(t)
	pool, err := InitPool(SetFactory(factory), SetIdleTimeout(time.Second*15))
	if err != nil {
		aet.Error(err)
		return
	}
	pool.GetActive()
}

func TestChannelPool_Handle(t *testing.T) {
	aet := assert.New(t)
	pool, err := InitPool(SetFactory(factory), SetIdleTimeout(time.Second*15))
	if err != nil {
		aet.Error(err)
		return
	}
	if err := pool.Handle(func(conn IConn) error {
		return conn.Use("hahahah")
	}); err != nil {
		aet.Error(err)
		return
	}
}
