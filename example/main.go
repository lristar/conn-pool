package main

import (
	"github.com/lristar/conn-pool"
	"math/rand"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	num int32
)

type Conn struct {
}

// Factory 生成连接的方法
func Factory() (pool.IConn, error) {
	return new(Conn), nil
}

// Ping 检查连接是否有效的方法
func (c *Conn) Ping() error {
	return nil
}

func (c *Conn) Close() error {
	return nil
}

func (c *Conn) Use(v interface{}) error {
	pool.Info("Use 调用了这个连接")
	return nil
}

const addr string = "127.0.0.1:8080"

func main() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGALRM)
	p, err := pool.NewChannelPool(pool.Config{
		InitialCap:  5,
		MaxCap:      20,
		Fac:         Factory,
		IdleTimeout: 20,
	})
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			time.Sleep(time.Second)
			i, j := p.GetActive()
			pool.Infof("现在积极连接：%d, 当前管道剩余连接数%d", i, j)

			pool.Infof("num is %d", atomic.LoadInt32(&num))
		}
	}()
	for i := 0; i < 10000; i++ {
		rand.Seed(time.Now().UnixNano())
		t := rand.Intn(50) + 1
		for j := 0; j < t; j++ {
			go func() {
				p, _ := pool.GetPool()
				if err := p.Handle(func(conn pool.IConn) error {
					atomic.AddInt32(&num, 1)
					_ = conn.Use("adfdafsf")
					return nil
				}); err != nil {
					pool.Errorf("err is %v", err)
				}
			}()
		}
		time.Sleep(time.Second * 15)
	}
	pool.Info("使用: ctrl+c 退出服务")
	defer func() { pool.Info("服务退出") }()
	<-c
}
