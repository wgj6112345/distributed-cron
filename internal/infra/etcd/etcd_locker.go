// internal/infra/etcd/etcd_locker.go
package etcd

import (
	"context"
	"fmt"
	"time"

	"distributed-cron/internal/domain" // 导入 domain 包
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

const (
	// LockPrefix 定义了 etcd 中分布式锁的根路径
	LockPrefix = "/cron/locks/"
	// LockSessionTTL 定义了锁会话的 TTL
	LockSessionTTL = 10 // seconds
)

// etcdLock 实现了 domain.Lock 接口
type etcdLock struct {
	mutex   *concurrency.Mutex // 底层的 etcd 分布式互斥锁
	session *concurrency.Session // 锁关联的 etcd 会话
	name    string             // 锁的名称
}

// Unlock 释放 etcd 分布式锁
func (l *etcdLock) Unlock(ctx context.Context) error {
	defer func() {
		// 确保在解锁后关闭会话，释放租约
		if l.session != nil {
			// concurrency.Session.Close() 是一个阻塞调用，需要一个 context
			// 这里使用 context.Background()，表示没有超时或取消。
			// 在实际生产中，可以考虑使用带超时或特定cancel的context。
			_ = l.session.Close() 
		}
	}()

	if err := l.mutex.Unlock(ctx); err != nil {
		return fmt.Errorf("failed to unlock %s: %w", l.name, err)
	}
	return nil
}


// etcdLocker 实现了 domain.Locker 接口
type etcdLocker struct {
	client *clientv3.Client
}

// NewEtcdLocker 创建一个新的 etcdLocker 实例
func NewEtcdLocker(client *clientv3.Client) domain.Locker {
	return &etcdLocker{client: client}
}

// Lock 尝试获取一个指定名称的分布式锁
func (l *etcdLocker) Lock(ctx context.Context, name string) (domain.Lock, error) {
	// 1. 创建一个新的会话 (Session)
	//    每个锁尝试都创建一个新会话，保证隔离性。
	//    如果会话关闭或租约过期，锁会自动释放。
	session, err := concurrency.NewSession(l.client, concurrency.WithTTL(LockSessionTTL))
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd session for lock %s: %w", name, err)
	}

	// 2. 使用会话创建一个 etcd 分布式互斥锁
	mutex := concurrency.NewMutex(session, LockPrefix+name)

	// 3. 尝试获取锁 (非阻塞尝试)
	//    context.WithTimeout 可以防止 TryLock 永远阻塞
	tryCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond) // 尝试获取锁100ms
	defer cancel() // 确保 cancel 函数被调用，释放 tryCtx 资源

	// TryLock 会尝试获取锁。如果锁已被持有，它会立即返回错误。
	if err := mutex.TryLock(tryCtx); err != nil {
		_ = session.Close() // 尝试失败，关闭会话
		if err == context.DeadlineExceeded {
			// 如果 TryLock 超时，说明锁在短时间内未被释放
			return nil, domain.ErrLockNotAcquired // 返回我们 domain 层定义的错误
		}
		return nil, fmt.Errorf("failed to try acquiring etcd lock %s: %w", name, err)
	}

	// 4. 成功获取锁，返回 etcdLock 实例
	return &etcdLock{
		mutex:   mutex,
		session: session,
		name:    name,
	}, nil
}
