package etcd

import (
	"context"
	"distributed-cron/internal/domain"
	"log/slog"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

const (
	LeaderElectionKey = "/cron/leader"
)

type etcdLeaderElectionManager struct {
	client   *clientv3.Client
	session  *concurrency.Session
	election *concurrency.Election
	isLeader bool
	mutex    sync.RWMutex
	nodeID   string // The ID of the current node
	ttl      time.Duration
	logger   *slog.Logger
}

// NewEtcdLeaderElectionManager creates a manager for leader election using etcd.
func NewEtcdLeaderElectionManager(client *clientv3.Client, nodeID string, ttl time.Duration, logger *slog.Logger) domain.LeaderElectionManager {
	return &etcdLeaderElectionManager{
		client: client,
		nodeID: nodeID,
		ttl:    ttl,
		logger: logger.With("component", "leader-election"),
	}
}

func (m *etcdLeaderElectionManager) Campaign(ctx context.Context) (<-chan struct{}, error) {
	var err error
	// Create a new session with a lease. If this node fails, the lease will expire.
	m.session, err = concurrency.NewSession(m.client, concurrency.WithTTL(int(m.ttl.Seconds()))) // Use configurable TTL
	if err != nil {
		return nil, err
	}

	// Create a new election with a specific key prefix.
	m.election = concurrency.NewElection(m.session, LeaderElectionKey)

	// Campaign blocks until this node becomes the leader or the context is canceled.
	if err := m.election.Campaign(ctx, m.nodeID); err != nil {
		return nil, err
	}

	m.logger.Info("successfully campaigned and became the leader", "node_id", m.nodeID)
	m.mutex.Lock()
	m.isLeader = true
	m.mutex.Unlock()

	// The returned channel is closed if the session expires, meaning leadership is lost.
	return m.session.Done(), nil
}

func (m *etcdLeaderElectionManager) Resign(ctx context.Context) error {
	m.mutex.Lock()
	m.isLeader = false
	m.mutex.Unlock()

	if m.election != nil {
		m.logger.Info("resigning leadership", "node_id", m.nodeID)
		return m.election.Resign(ctx)
	}
	return nil
}

func (m *etcdLeaderElectionManager) IsLeader() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.isLeader
}
