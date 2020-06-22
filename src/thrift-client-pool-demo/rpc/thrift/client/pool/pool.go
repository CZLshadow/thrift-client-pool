package pool

import (
	"container/list"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
)

// Thrift客户端创建方法，留给业务去实现
type ThriftDial func(addr string, connTimeout time.Duration) (*IdleClient, error)

// 关闭Thrift客户端，留给业务实现
type ThriftClientClose func(c *IdleClient) error

// Thrift客户端连接池
type ThriftPool struct {
	// Thrift客户端创建逻辑，业务自己实现
	Dial ThriftDial
	// Thrift客户端关闭逻辑，业务自己实现
	Close ThriftClientClose
	// 空闲客户端，用双端队列存储
	idle list.List
	// 同步锁，确保count、status、idle等公共数据并发操作安全
	lock *sync.Mutex
	// 记录当前已经创建的Thrift客户端，确保MaxConn配置
	count int32
	// Thrift客户端连接池状态，目前就open和stop两种
	status uint32
	// Thrift客户端连接池相关配置
	config *ThriftPoolConfig
}

// 连接池配置
type ThriftPoolConfig struct {
	// Thrfit Server端地址
	Addr string
	// 最大连接数
	MaxConn int32
	// 创建连接超时时间
	ConnTimeout time.Duration
	// 空闲客户端超时时间，超时主动释放连接，关闭客户端
	IdleTimeout time.Duration
	// 获取Thrift客户端超时时间
	Timeout time.Duration
	// 获取Thrift客户端失败重试间隔
	interval time.Duration
}

// Thrift客户端
type IdleClient struct {
	// Thrift传输层，封装了底层连接建立、维护、关闭、数据读写等细节
	Transport thrift.TTransport
	// 真正的Thrift客户端，业务创建传入
	RawClient interface{}
}

// 封装了Thrift客户端
type idleConn struct {
	// 空闲Thrift客户端
	c *IdleClient
	// 最近一次放入空闲队列的时间
	t time.Time
}

const (
	CHECKINTERVAL = 120 //清除超时连接间隔

	poolOpen = 1
	poolStop = 2

	DEFAULT_MAX_CONN     = 60
	DEFAULT_CONN_TIMEOUT = time.Second * 2
	DEFAULT_IDLE_TIMEOUT = time.Minute * 15
	maxInitConnCount     = 10
	DEFAULT_TIMEOUT      = time.Second * 5
	defaultInterval      = time.Millisecond * 50
)

var nowFunc = time.Now

//error
var (
	ErrOverMax          = errors.New("ThriftPool 连接超过设置的最大连接数")
	ErrInvalidConn      = errors.New("ThriftPool Client回收时变成nil")
	ErrPoolClosed       = errors.New("ThriftPool 连接池已经被关闭")
	ErrSocketDisconnect = errors.New("ThriftPool 客户端socket连接已断开")
)

func NewThriftPool(config *ThriftPoolConfig, dial ThriftDial, closeFunc ThriftClientClose) *ThriftPool {
	// 检查连接池配置
	checkThriftConfig(config)
	thriftPool := &ThriftPool{
		Dial:   dial,
		Close:  closeFunc,
		lock:   &sync.Mutex{},
		config: config,
		status: poolOpen,
		count:  0,
	}
	// 初始化空闲链接
	thriftPool.initConn()
	// 定期清理过期空闲连接
	go thriftPool.ClearConn()
	return thriftPool
}

func checkThriftConfig(config *ThriftPoolConfig) {
	if config.MaxConn == 0 {
		config.MaxConn = DEFAULT_MAX_CONN
	}
	if config.ConnTimeout == 0 {
		config.ConnTimeout = DEFAULT_CONN_TIMEOUT
	}
	if config.IdleTimeout <= 0 {
		config.IdleTimeout = DEFAULT_IDLE_TIMEOUT
	}
	if config.Timeout <= 0 {
		config.Timeout = DEFAULT_TIMEOUT
	}
	config.interval = defaultInterval
}

// 获取Thrift空闲客户端
func (p *ThriftPool) Get() (*IdleClient, error) {
	return p.get(nowFunc().Add(p.config.Timeout))
}

// 获取连接的逻辑实现
// expire设定了一个超时时间点，当没有可用连接时，程序会休眠一小段时间后重试
// 如果一直获取不到连接，一旦到达超时时间点，则报ErrOverMax错误
func (p *ThriftPool) get(expire time.Time) (*IdleClient, error) {
	if atomic.LoadUint32(&p.status) == poolStop {
		return nil, ErrPoolClosed
	}

	// 判断是否超额
	p.lock.Lock()
	if p.idle.Len() == 0 && atomic.LoadInt32(&p.count) >= p.config.MaxConn {
		p.lock.Unlock()
		// 不采用递归的方式来实现重试机制，防止栈溢出，这里改用循环方式来实现重试
		for {
			// 休眠一段时间再重试
			time.Sleep(p.config.interval)
			// 超时退出
			if nowFunc().After(expire) {
				return nil, ErrOverMax
			}
			p.lock.Lock()
			if p.idle.Len() == 0 && atomic.LoadInt32(&p.count) >= p.config.MaxConn {
				p.lock.Unlock()
			} else { // 有可用链接，退出for循环
				break
			}
		}
	}

	if p.idle.Len() == 0 {
		// 先加1，防止首次创建连接时，TCP握手太久，导致p.count未能及时+1，而新的请求已经到来
		// 从而导致短暂性实际连接数大于p.count（大部分链接由于无法进入空闲链接队列，而被关闭，处于TIME_WATI状态）
		atomic.AddInt32(&p.count, 1)
		p.lock.Unlock()
		client, err := p.Dial(p.config.Addr, p.config.ConnTimeout)
		if err != nil {
			atomic.AddInt32(&p.count, -1)
			return nil, err
		}
		// 检查连接是否有效
		if !client.Check() {
			atomic.AddInt32(&p.count, -1)
			return nil, ErrSocketDisconnect
		}

		return client, nil
	}

	// 从队头中获取空闲连接
	ele := p.idle.Front()
	idlec := ele.Value.(*idleConn)
	p.idle.Remove(ele)
	p.lock.Unlock()

	// 连接从空闲队列获取，可能已经关闭了，这里再重新检查一遍
	if !idlec.c.Check() {
		atomic.AddInt32(&p.count, -1)
		return nil, ErrSocketDisconnect
	}
	return idlec.c, nil
}

// 归还Thrift客户端
func (p *ThriftPool) Put(client *IdleClient) error {
	if client == nil {
		return nil
	}

	if atomic.LoadUint32(&p.status) == poolStop {
		err := p.Close(client)
		client = nil
		return err
	}

	if atomic.LoadInt32(&p.count) > p.config.MaxConn || !client.Check() {
		atomic.AddInt32(&p.count, -1)
		err := p.Close(client)
		client = nil
		return err
	}

	p.lock.Lock()
	p.idle.PushFront(&idleConn{
		c: client,
		t: nowFunc(),
	})
	p.lock.Unlock()

	return nil
}

// 关闭有问题的连接，并创建新的连接
func (p *ThriftPool) Reconnect(client *IdleClient) (newClient *IdleClient, err error) {
	if client != nil {
		p.Close(client)
	}
	client = nil

	newClient, err = p.Dial(p.config.Addr, p.config.ConnTimeout)
	if err != nil {
		atomic.AddInt32(&p.count, -1)
		return
	}
	if !newClient.Check() {
		atomic.AddInt32(&p.count, -1)
		return nil, ErrSocketDisconnect
	}
	return
}

func (p *ThriftPool) CloseConn(client *IdleClient) {
	if client != nil {
		p.Close(client)
	}
	atomic.AddInt32(&p.count, -1)
}

func (p *ThriftPool) CheckTimeout() {
	p.lock.Lock()
	for p.idle.Len() != 0 {
		ele := p.idle.Back()
		if ele == nil {
			break
		}
		v := ele.Value.(*idleConn)
		if v.t.Add(p.config.IdleTimeout).After(nowFunc()) {
			break
		}
		//timeout && clear
		p.idle.Remove(ele)
		p.lock.Unlock()

		p.Close(v.c) //close client connection
		atomic.AddInt32(&p.count, -1)

		p.lock.Lock()
	}
	p.lock.Unlock()
	return
}

// 检测连接是否有效
func (c *IdleClient) Check() bool {
	if c.Transport == nil || c.RawClient == nil {
		return false
	}
	return c.Transport.IsOpen()
}

func (p *ThriftPool) GetIdleCount() uint32 {
	if p != nil {
		return uint32(p.idle.Len())
	}
	return 0
}

func (p *ThriftPool) GetConnCount() int32 {
	if p != nil {
		return atomic.LoadInt32(&p.count)
	}
	return 0
}

func (p *ThriftPool) ClearConn() {
	sleepTime := CHECKINTERVAL * time.Second
	if sleepTime < p.config.IdleTimeout {
		sleepTime = p.config.IdleTimeout
	}
	for {
		p.CheckTimeout()
		time.Sleep(CHECKINTERVAL * time.Second)
	}
}

func (p *ThriftPool) Release() {
	atomic.StoreUint32(&p.status, poolStop)
	atomic.StoreInt32(&p.count, 0)

	p.lock.Lock()
	idle := p.idle
	p.idle.Init()
	p.lock.Unlock()

	for iter := idle.Front(); iter != nil; iter = iter.Next() {
		p.Close(iter.Value.(*idleConn).c)
	}
}

func (p *ThriftPool) Recover() {
	atomic.StoreUint32(&p.status, poolOpen)
}

// 链接池创建时，先初始化一定数量的空闲连接数
func (p *ThriftPool) initConn() {
	initCount := p.config.MaxConn
	if initCount > maxInitConnCount {
		initCount = maxInitConnCount
	}
	wg := &sync.WaitGroup{}
	wg.Add(int(initCount))
	for i := int32(0); i < initCount; i++ {
		go p.createIdleConn(wg)
	}
	wg.Wait()
}

func (p *ThriftPool) createIdleConn(wg *sync.WaitGroup) {
	c, _ := p.Get()
	p.Put(c)
	wg.Done()
}
