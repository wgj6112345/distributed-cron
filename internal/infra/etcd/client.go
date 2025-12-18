package etcd

import (
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func NewClient(endpoints []string, timeout time.Duration) (*clientv3.Client, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: timeout,
	})
	if err != nil {
		return nil, err
	}
	return cli, nil
}
