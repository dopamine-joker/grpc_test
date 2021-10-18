package client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/prometheus/common/log"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/resolver"
	"grpc_test/grpc_demo/service"
	"strings"
	"time"
)

const (
	schema     = "etcd"
	BasePath   = "go_test"
	ServerPath = "grpc_demo"
)

type Resolver struct {
	EtcdAddrs  []string
	DiaTimeout int
	GetTimeout int

	keyPrefix  string
	basePath   string
	serverPath string
	schema     string
	srvAddrs   []resolver.Address

	watchCh clientv3.WatchChan
	closeCh chan struct{}

	client *clientv3.Client
	cc     resolver.ClientConn
}

func NewResolver(etcdAddrs []string, basePath, serverPath string, diaTimeout, getTimeOut int) (*Resolver, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   etcdAddrs,
		DialTimeout: time.Duration(diaTimeout) * time.Second,
	})
	if err != nil {
		return nil, err
	}

	return &Resolver{
		EtcdAddrs:  etcdAddrs,
		DiaTimeout: diaTimeout,
		GetTimeout: getTimeOut,
		basePath:   basePath,
		serverPath: serverPath,
		schema:     schema,
		closeCh:    make(chan struct{}),
		client:     client,
	}, nil

}

func (r *Resolver) ResolveNow(options resolver.ResolveNowOptions) {
	log.Info("ResolveNow")
}

func (r *Resolver) Close() {
	r.closeCh <- struct{}{}
}

func (r *Resolver) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r.cc = cc
	r.keyPrefix = r.buildPrefix()
	if err := r.start(); err != nil {
		return nil, err
	}
	return r, nil

}

func (r *Resolver) Scheme() string {
	return r.schema
}

func (r *Resolver) start() error {
	resolver.Register(r)

	if err := r.sync(); err != nil {
		return err
	}

	go r.watch()

	return nil
}

func (r *Resolver) watch() {
	ticker := time.NewTicker(time.Minute)
	r.watchCh = r.client.Watch(context.Background(), r.keyPrefix, clientv3.WithPrefix())
	for {
		select {
		case <-r.closeCh:
			return
		case res, ok := <-r.watchCh:
			if ok {
				r.update(res.Events)
			}
		case <-ticker.C:
			if err := r.sync(); err != nil {
				log.Error("sync failed", err)
			}

		}
	}
}

func (r *Resolver) update(event []*clientv3.Event) {
	for _, ev := range event {
		switch ev.Type {
		case mvccpb.PUT:
			srv, err := r.parseValue(ev.Kv.Value)
			if err != nil {
				log.Error("json unmarshal err", err)
				continue
			}
			addr := resolver.Address{
				Addr: srv.Host,
			}
			if !Exist(r.srvAddrs, addr) {
				r.srvAddrs = append(r.srvAddrs, addr)
				_ = r.cc.UpdateState(resolver.State{
					Addresses: r.srvAddrs,
				})
			}
		case mvccpb.DELETE:
			srvAddr := strings.TrimPrefix(string(ev.Kv.Key), r.keyPrefix)
			addr := resolver.Address{
				Addr: srvAddr,
			}
			if s, ok := Remove(r.srvAddrs, addr); ok {
				r.srvAddrs = s
				_ = r.cc.UpdateState(resolver.State{
					Addresses: r.srvAddrs,
				})
			}
		}
	}
}

func (r *Resolver) sync() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(r.GetTimeout)*time.Second)
	defer cancel()
	res, err := r.client.Get(ctx, r.keyPrefix, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, v := range res.Kvs {
		srv, err := r.parseValue(v.Value)
		if err != nil {
			log.Error("json unmarshal data err", err, string(v.Value))
		}
		addr := resolver.Address{
			Addr: srv.Host,
		}
		r.srvAddrs = append(r.srvAddrs, addr)
	}
	if err = r.cc.UpdateState(resolver.State{Addresses: r.srvAddrs}); err != nil {
		return err
	}
	return nil
}

func (r *Resolver) parseValue(value []byte) (service.UserService, error) {
	var err error
	srv := service.UserService{}
	if err = json.Unmarshal(value, &srv); err != nil {
		return srv, err
	}
	return srv, nil
}

func (r *Resolver) buildPrefix() string {
	return fmt.Sprintf("/%s/%s/", r.basePath, r.serverPath)
}

func Exist(srvAddrs []resolver.Address, addr resolver.Address) bool {
	for _, srvAddr := range srvAddrs {
		if srvAddr.Addr == addr.Addr {
			return true
		}
	}
	return false
}

// Remove helper function
func Remove(s []resolver.Address, addr resolver.Address) ([]resolver.Address, bool) {
	for i := range s {
		if s[i].Addr == addr.Addr {
			s[i] = s[len(s)-1]
			return s[:len(s)-1], true
		}
	}
	return nil, false
}
