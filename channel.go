package pool

import (
	"errors"
	"fmt"
	"sync"
	"time"
	//"reflect"
)

var (
	//ErrMaxActiveConnReached 连接池超限
	ErrMaxActiveConnReached = errors.New("MaxActiveConnReached")
	ErrConnClosed           = errors.New("连接管道关闭")
	ErrGetConnTimeOut       = errors.New("获取连接超时")
	ErrConnZero             = errors.New("连接池中的连接为空")
	//ErrClosed 连接池已经关闭Error
	ErrClosed = errors.New("pool is closed")
	// 超时时间
	DelayTime10s = time.Second * 10
)

type IFactory interface {
	// Factory 生成连接的方法
	Factory() (interface{}, error)
	// Close 关闭连接的方法
	Close(interface{}) error
	// Ping 检查连接是否有效的方法
	Ping(interface{}) error
}

// Config 连接池相关配置
type Config struct {
	//连接池中拥有的最小连接数
	InitialCap int
	//最大并发存活连接数
	MaxCap int
	Fac    IFactory
	//连接最大空闲时间，超过该事件则将失效
	IdleTimeout time.Duration
}

// channelPool 存放连接信息
type channelPool struct {
	Config
	mu                       sync.RWMutex
	fac                      IFactory
	conns                    chan *idleConn
	idleTimeout, waitTimeOut time.Duration
	maxActive                int
	openingConns             int
}

// NewChannelPool 初始化连接
func NewChannelPool(poolConfig Config) (*channelPool, error) {
	if poolConfig.InitialCap > poolConfig.MaxCap || poolConfig.InitialCap <= 0 {
		return nil, errors.New("invalid capacity settings")
	}

	if poolConfig.Fac == nil {
		return nil, fmt.Errorf("没有工厂，无法创建")
	}

	c := &channelPool{
		Config:       poolConfig,
		conns:        make(chan *idleConn, poolConfig.MaxCap),
		fac:          poolConfig.Fac,
		idleTimeout:  poolConfig.IdleTimeout,
		maxActive:    poolConfig.MaxCap,
		openingConns: 0,
	}

	for i := 0; i < poolConfig.InitialCap; i++ {
		conn, err := c.factory(false)
		if err != nil {
			c.Release()
			return nil, fmt.Errorf("factory is not able to fill the pool: %s", err)
		}
		if err = c.put(conn); err != nil {
			// 如果入管道失败就直接创建失败
			_ = c.Close()
			return nil, err
		}
	}
	return c, nil
}

// getConns 获取所有连接
func (c *channelPool) getConns() chan *idleConn {
	c.mu.Lock()
	conns := c.conns
	c.mu.Unlock()
	return conns
}

// Handle 从pool内获取连接使用同时放回
func (c *channelPool) Handle(handle func(interface{}) error) error {
	conns := c.getConns()
	if conns == nil {
		return ErrClosed
	}
	var (
		cc  *idleConn // 单个连接
		err error
	)
	for {
		select {
		case wrapConn, ok := <-conns:
			if !ok {
				return ErrClosed
			}
			if c.isRelease(wrapConn) {
				continue
			}
			cc = wrapConn
			goto run
		default:
			c.mu.RLock()
			connectionNum := c.openingConns
			c.mu.RUnlock()
			Infof("openConn %v %v", connectionNum, c.maxActive)
			if connectionNum >= c.maxActive {
				// 如果达到最大数量的队列了，就等待是否有回收的可以用
				cc, err = c.factory(true)
				if err != nil {
					return err
				}
			} else {
				cc, err = c.factory(false)
				if err != nil {
					return err
				}
			}
			goto run
		}
	}
run:
	defer c.put(cc)
	return handle(cc)
}

// Put 将连接放回pool中 这个集成到get里面
func (c *channelPool) put(conn *idleConn) error {
	if conn == nil {
		return errors.New("connection is nil. rejecting")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conns == nil {
		return c.closeOne(conn)
	}
	num := c.openingConns
	if num >= c.maxActive {
		return c.closeOne(conn)
	}
	// 得判断管道是否满了
	select {
	case c.conns <- conn:
	default:
		return c.closeOne(conn)
	}
	return nil
}

// Close 关闭单条连接
func (c *channelPool) closeOne(conn *idleConn) error {
	if conn == nil {
		return errors.New("connection is nil. rejecting")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.openingConns--
	return c.fac.Close(conn.conn)
}

// ReleaseOne 判断如果需要释放就释放
func (c *channelPool) isRelease(conn *idleConn) bool {
	//判断是否超时，超时则丢弃
	if timeout := c.idleTimeout; timeout > 0 {
		if conn.timeOut(timeout) {
			//丢弃并关闭该连接
			_ = c.closeOne(conn)
			return true
		}
	}
	//判断是否失效，失效则丢弃
	if err := c.ping(conn); err != nil {
		_ = c.closeOne(conn)
		return true
	}
	return false
}

// Ping 检查单条连接是否有效
func (c *channelPool) ping(conn *idleConn) error {
	if conn == nil {
		return errors.New("connection is nil. rejecting")
	}
	return c.fac.Ping(conn.conn)
}

// 获取连接，如果没有就重建新连接（如果满了就等待连接池释放的连接，超过十秒报错）
func (c *channelPool) factory(wait bool) (*idleConn, error) {
	tc := time.NewTicker(DelayTime10s)
	if wait {
		for {
			select {
			case conn, ok := <-c.conns:
				if ok {
					if c.isRelease(conn) {
						continue
					} else {
						return conn, nil
					}
				} else {
					return nil, ErrConnClosed
				}
				// 超时退出
			case <-tc.C:
				tc.Stop()
				return nil, ErrGetConnTimeOut
			}
		}
	} else {
		conn, err := c.fac.Factory()
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		var idle *idleConn
		if c.openingConns < c.InitialCap {
			idle = newIdleConn(conn, true)
		} else {
			idle = newIdleConn(conn, false)
		}
		defer func() { c.openingConns++ }()
		defer c.mu.Unlock()
		// 保证含有最少连接
		return idle, nil
	}
}

// Release 释放连接池中所有连接 不关闭管道
func (c *channelPool) Release() {
	c.mu.Lock()
	conns := c.conns
	c.conns = nil
	closeFun := c.fac.Close
	c.mu.Unlock()
	if conns == nil {
		return
	}
	for wrapConn := range conns {
		_ = closeFun(wrapConn.conn)
	}
}

// Close 关闭管道并且释放连接
func (c *channelPool) Close() error {
	c.mu.Lock()
	conns := c.conns
	c.conns = nil
	closeFun := c.fac.Close
	c.mu.Unlock()
	if conns == nil {
		return nil
	}
	close(c.conns)
	for wrapConn := range conns {
		_ = closeFun(wrapConn.conn)
	}
	return nil
}

// Len 连接池中已有的连接
func (c *channelPool) Len() int {
	return len(c.getConns())
}

func (c *channelPool) GetActive() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.openingConns
}
