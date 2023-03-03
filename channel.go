package pool

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	//ErrMaxActiveConnReached 连接池超限
	ErrMaxActiveConnReached = errors.New("MaxActiveConnReached")
	ErrConnClosed           = errors.New("连接管道关闭")
	ErrGetConnTimeOut       = errors.New("获取连接超时")
	ErrConnZero             = errors.New("连接池中的连接为空")
	//ErrClosed 连接池已经关闭Error
	ErrClosed = errors.New("pool is closed")
	// DelayTime10s 超时时间
	DelayTime10s = time.Second * 10
	// 公用连接池
	pools *ChannelPool
)

type (
	IConn interface {
		// Close 关闭连接的方法
		Close() error
		// Ping 检查连接是否有效的方法
		Ping() error
		// Use 应用这个连接
		Use(interface{}) error
	}
	Fac func() (IConn, error)
)

// Config 连接池相关配置
type (
	Option func(opt *Config)
	Config struct {
		//连接池中拥有的最小连接数
		InitialCap int32
		//最大并发存活连接数
		MaxCap int32
		Fac    Fac
		//连接最大空闲时间，超过该事件则将失效
		IdleTimeout time.Duration
	}
	// ChannelPool 存放连接信息
	ChannelPool struct {
		Config
		mu                       *sync.RWMutex
		fac                      Fac
		conns                    chan *idleConn
		idleTimeout, waitTimeOut time.Duration
		maxActive                int32
		openingConns             int32
		isClose                  bool
	}
)

func SetInitialCap(initNum int32) Option {
	return func(opt *Config) {
		opt.InitialCap = initNum
	}
}

func SetMaxCap(maxNum int32) Option {
	return func(opt *Config) {
		opt.InitialCap = maxNum
	}
}

func SetIdleTimeout(timeExpire time.Duration) Option {
	return func(opt *Config) {
		opt.IdleTimeout = timeExpire
	}
}

func SetFactory(f Fac) Option {
	return func(opt *Config) {
		opt.Fac = f
	}
}

func newConfig() *Config {
	return &Config{
		InitialCap:  5,
		MaxCap:      20,
		Fac:         nil,
		IdleTimeout: time.Second * 20,
	}
}

// InitPool 初始化连接池
func InitPool(ops ...Option) (*ChannelPool, error) {
	poolConfig := newConfig()
	for i := range ops {
		ops[i](poolConfig)
	}
	if poolConfig.InitialCap > poolConfig.MaxCap || poolConfig.InitialCap <= 0 {
		return nil, errors.New("invalid capacity settings")
	}

	if poolConfig.Fac == nil {
		return nil, fmt.Errorf("没有工厂，无法创建")
	}

	pools = &ChannelPool{
		mu:           &sync.RWMutex{},
		Config:       *poolConfig,
		conns:        make(chan *idleConn, poolConfig.MaxCap),
		fac:          poolConfig.Fac,
		idleTimeout:  poolConfig.IdleTimeout,
		maxActive:    poolConfig.MaxCap,
		openingConns: 0,
	}

	for i := 0; i < int(poolConfig.InitialCap); i++ {
		conn, err := pools.factory(false)
		if err != nil {
			pools.Release()
			return nil, fmt.Errorf("factory is not able to fill the pool: %s", err)
		}
		if err = pools.put(conn); err != nil {
			// 如果入管道失败就直接创建失败
			_ = pools.Close()
			return nil, err
		}
	}
	return pools, nil
}

func GetPool() (*ChannelPool, error) {
	if pools == nil {
		return nil, fmt.Errorf("连接池为空")
	}
	return pools, nil
}

// Handle 从pool内获取连接使用同时放回
func (c *ChannelPool) Handle(handle func(conn IConn) error) error {
	var (
		cc  *idleConn // 单个连接
		err error
	)
	if c.isClose {
		return nil
	}
	wrapConn, err := c.getConnNoWait()
	if err == nil {
		Infof("获取旧连接成功")
		cc = wrapConn
	} else {
		cc, err = c.factory(false)
		if err != nil {
			return err
		}
	}
	defer func() { _ = c.put(cc) }()
	return cc.handle(handle)
}

// 获取连接，如果没有就重建新连接（如果满了就等待连接池释放的连接，超过十秒报错）
func (c *ChannelPool) factory(wait bool) (*idleConn, error) {
	if wait {
		return c.getConnWait(DelayTime10s)
	} else {
		c.mu.Lock()
		if c.openingConns < c.maxActive {
			conn, err := c.fac()
			if err != nil {
				c.mu.Unlock()
				return c.factory(true)
			}
			defer c.mu.Unlock()
			defer Infof("获取新连接成功 --%d", c.openingConns)
			defer atomic.AddInt32(&c.openingConns, 1)
			if c.openingConns < c.InitialCap {
				return newIdleConn(conn, true), nil
			}
			return newIdleConn(conn, false), nil
		} else {
			c.mu.Unlock()
			return c.factory(true)
		}
	}
}

// GetConnWait 等待获取连接
func (c *ChannelPool) getConnWait(waitTime time.Duration) (conn *idleConn, err error) {
	tc := time.NewTicker(waitTime)
	for {
		select {
		case con := <-c.conns:
			if c.isRelease(con) {
				continue
			} else {
				return con, nil
			}
		case <-tc.C:
			tc.Stop()
			return nil, fmt.Errorf("获取超时")
		}
	}
}

func (c *ChannelPool) getConnNoWait() (conn *idleConn, err error) {
	for {
		select {
		case con := <-c.conns:
			if c.isRelease(con) {
				continue
			} else {
				return con, nil
			}
		default:
			return nil, fmt.Errorf("未获取到旧连接")
		}
	}
}

// Put 将连接放回pool中 这个集成到get里面
func (c *ChannelPool) put(conn *idleConn) error {
	if conn == nil {
		return errors.New("connection is nil. rejecting")
	}
	// 续费
	defer func() {
		if err := recover(); err != nil {
			c.closeOne(conn)
		}
	}()
	conn.t = time.Now()
	// 得判断管道是否满了
	select {
	case c.conns <- conn:
	default:
		return c.closeOne(conn)
	}
	return nil
}

// Close 关闭单条连接
func (c *ChannelPool) closeOne(conn *idleConn) error {
	if conn == nil {
		return errors.New("connection is nil. rejecting")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	atomic.AddInt32(&c.openingConns, -1)
	return conn.conn.Close()
}

// ReleaseOne 判断如果需要释放就释放
func (c *ChannelPool) isRelease(conn *idleConn) bool {
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
func (c *ChannelPool) ping(conn *idleConn) error {
	if conn == nil {
		return errors.New("connection is nil. rejecting")
	}
	return conn.conn.Ping()
}

// Release 释放连接池中所有连接 不关闭管道
func (c *ChannelPool) Release() {
	for {
		select {
		case wrapConn := <-c.conns:
			_ = c.closeOne(wrapConn)
		default:
			return
		}
	}
}

// Close 关闭管道并且释放连接
func (c *ChannelPool) Close() error {
	c.mu.Lock()
	defer func() {
		if err := recover(); err != nil {
			Error(err)
		}
	}()
	close(c.conns)
	c.isClose = true
	c.mu.Unlock()
	for conn := range c.conns {
		_ = c.closeOne(conn)
	}
	return nil
}

func (c *ChannelPool) GetActive() (int32, int) {
	return c.openingConns, len(c.conns)
}
