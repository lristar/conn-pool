package main

import (
	"fmt"
	"github.com/lristar/pool"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Server struct {
}

func (s *Server) Close(v interface{}) error {
	fmt.Println("关闭一个连接")
	return nil
}

// Factory 生成连接的方法
func (s *Server) Factory() (interface{}, error) {
	pool.Info("创建了一个连接")
	return s, nil
}

// Ping 检查连接是否有效的方法
func (s *Server) Ping(interface{}) error {
	pool.Info("ping了一下")
	return nil
}

func handle(s interface{}) error {
	pool.Info("处理了一次")
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
		Fac:         new(Server),
		IdleTimeout: 200,
	})
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			time.Sleep(time.Second)
			pool.Info("现在积极连接：", p.GetActive())
		}
	}()
	for i := 0; i < 10000; i++ {
		for j := 0; j < 6; j++ {
			go func() {
				rand.Seed(time.Now().UnixNano())
				t := rand.Intn(8) + 1
				pool.Info("t is ", t)
				time.Sleep(time.Duration(t) * time.Second)
				p.Handle(handle)
			}()
		}
		time.Sleep(time.Second * 10)
	}
	pool.Info("使用: ctrl+c 退出服务")
	defer func() { pool.Info("服务退出") }()
	<-c
}

func server() {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println("Error listening: ", err)
		os.Exit(1)
	}
	defer l.Close()
	fmt.Println("Listening on ", addr)
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err)
		}
		fmt.Printf("Received message %s -> %s \n", conn.RemoteAddr(), conn.LocalAddr())
		//go handleRequest(conn)
	}
}
