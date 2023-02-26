package main

import (
	"github.com/lristar/pool"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Conn struct {
}

// Factory 生成连接的方法
func Factory() (pool.IConn, error) {
	pool.Info("创建了一个连接")
	return new(Conn), nil
}

// todo 这里的interface如何解决
// Ping 检查连接是否有效的方法
func (c *Conn) Ping() error {
	pool.Info("Ping 这个连接")
	return nil
}

func (c *Conn) Close() error {
	pool.Info("Close 关闭了这个连接")
	return nil
}

func (c *Conn) Use() error {
	pool.Info("Use 调用了这个连接")
	return nil
}

const addr string = "127.0.0.1:8080"

func main() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGALRM)
	//等待tcp server启动
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
		}
	}()
	for i := 0; i < 10000; i++ {
		rand.Seed(time.Now().UnixNano())
		t := rand.Intn(50) + 1
		pool.Info("t is ", t)
		for j := 0; j < t; j++ {
			go func() {
				if err := p.Handle(func(conn pool.IConn) error {
					_ = conn.Use()
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
