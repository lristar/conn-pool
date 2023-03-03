# conn-pool
#### 以此处为基础优化：git@github.com:silenceper/pool.git
#### 可直接下载使用或以此进行优化

## download
```
go get github.com/lristar/conn-pool
```
## Basic Usage:

```go
package main
import (
	pool "github.com/lristar/conn-pool"
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

func (c *Conn) Use(interface{}) error {
	pool.Info("Use 调用了这个连接")
	return nil
}
func main() {
	p, err := pool.InitPool(pool.SetFactory(Factory))
	if err!=nil{
		pool.Errorf("err is %v", err)
		return 
    }
	if err := p.Handle(func(conn pool.IConn) error {
		_ = conn.Use("hahaah")
		return nil
	}); err != nil {
		pool.Errorf("err is %v", err)
	}
}
```
