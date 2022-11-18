package transport

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/busgo/elsa/pkg/cmap"
	"github.com/busgo/elsa/pkg/common"
	"github.com/busgo/elsa/pkg/log"
	v3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"google.golang.org/grpc/connectivity"
)

type RegistryTransport interface {
	// Register  register a service
	Register(context.Context, common.URL) error

	// Unregister unregister a service
	Unregister(context.Context, common.URL) error

	// Subscribe subscribe a service
	Subscribe(context.Context, common.URL, common.NotifyListener) error

	// Unsubscribe unsubscribbe a service
	Unsubscribe(context.Context, common.URL, common.NotifyListener) error

	Recover() <-chan struct{}

	io.Closer
}

type registryTransport struct {
	cli     *v3.Client
	session *concurrency.Session
	opts    config

	watchers cmap.ConcurrentMap

	connected    bool
	sessionState bool

	recover           chan struct{}
	cancelFunc        context.CancelFunc
	sessionCancelFunc context.CancelFunc
}

func New(opts ...Option) (RegistryTransport, error) {
	cc := _config
	for _, opt := range opts {
		opt(&cc)
	}

	cli, err := v3.New(v3.Config{
		Endpoints:   cc.endpoints,
		Username:    cc.username,
		Password:    cc.password,
		DialTimeout: time.Second * time.Duration(cc.dialTimeout),
	})
	if err != nil {
		return nil, err
	}

	session, err := concurrency.NewSession(cli, concurrency.WithTTL(int(cc.ttl)))

	if err != nil {
		return nil, err
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	t := &registryTransport{
		cli:          cli,
		opts:         cc,
		session:      session,
		watchers:     cmap.New(),
		connected:    true,
		sessionState: true,
		cancelFunc:   cancelFunc,
		recover:      make(chan struct{}),
	}

	go t.loop(ctx)
	go t.watchSession()

	return t, nil
}

func (t *registryTransport) loop(ctx context.Context) {

	for {
		select {

		case <-ctx.Done():
			return
		case <-time.After(time.Second * 5):
			t.check()
		}
	}
}

func (t *registryTransport) check() {

	connected := t.isConnected()
	if t.connected != connected {

		if connected {
			log.Warnf("the registry transport is reconnected to %s", strings.Join(t.opts.endpoints, ","))
		} else {
			log.Warnf("the registry transport is disconnected to %s", strings.Join(t.opts.endpoints, ","))
		}
		t.connected = connected
	}

	t.refreshSession()
}

// isConnected  check the etcd client connection state
func (t *registryTransport) isConnected() bool {
	return t.cli.ActiveConnection().GetState() == connectivity.Ready || t.cli.ActiveConnection().GetState() == connectivity.Idle
}

// watchSession watch the session expired state
func (t *registryTransport) watchSession() {

	ctx, cancelFunc := context.WithCancel(context.Background())
	t.sessionCancelFunc = cancelFunc
	for {

		select {
		case <-ctx.Done():
			return
		case <-t.session.Done():
			t.sessionState = false
			t.refreshSession()
			return
		}
	}
}

func (t *registryTransport) refreshSession() {
	if !t.connected || t.sessionState {
		return
	}

	session, err := concurrency.NewSession(t.cli, concurrency.WithTTL(int(t.opts.ttl)))
	if err != nil {
		log.Infof("failed to refresh session, waitfing for retry, cause: %s", err.Error())
		return
	}

	t.session = session
	t.sessionState = true
	go t.watchSession()
	log.Infof("the refresh a new session success")
	t.recover <- struct{}{}
}

// Register  register a service
func (t *registryTransport) Register(ctx context.Context, u common.URL) error {
	urlPath := u.ToUrlPath()
	_, err := t.cli.Put(ctx, urlPath, u.String(), v3.WithLease(t.session.Lease()))
	return err
}

// Unregister unregister a service
func (t *registryTransport) Unregister(ctx context.Context, u common.URL) error {
	urlPath := u.ToUrlPath()
	_, err := t.cli.Delete(ctx, urlPath)
	return err
}

// Subscribe subscribe a service
func (t *registryTransport) Subscribe(ctx context.Context, u common.URL, l common.NotifyListener) error {

	watch, ok := t.watchers.Get(u.String())
	if ok {
		_ = watch.(*subscribeWatcher).Close()
	}
	watch, err := NewWatcher(ctx, u, l, t.cli)
	if err != nil {
		return err
	}
	t.watchers.Set(u.String(), watch)
	return nil
}

// Unsubscribe unsubscribbe a service
func (t *registryTransport) Unsubscribe(ctx context.Context, u common.URL, l common.NotifyListener) error {
	watch, ok := t.watchers.Get(u.String())
	if ok {
		return watch.(*subscribeWatcher).Close()
	}
	return nil
}

func (t *registryTransport) Recover() <-chan struct{} {
	return t.recover
}

func (t *registryTransport) Close() error {
	t.cancelFunc()
	t.sessionCancelFunc()

	items := t.watchers.Items()

	for _, v := range items {
		_ = v.(*subscribeWatcher).Close()
	}

	return nil
}

type subscribeWatcher struct {
	u          common.URL
	l          common.NotifyListener
	cli        *v3.Client
	path       string
	watcher    v3.Watcher
	urls       cmap.ConcurrentMap
	cancelFunc context.CancelFunc
	io.Closer
}

func NewWatcher(ctx context.Context, u common.URL, l common.NotifyListener, cli *v3.Client) (*subscribeWatcher, error) {

	watcher := v3.NewWatcher(cli)
	w := &subscribeWatcher{
		u:       u,
		l:       l,
		path:    fmt.Sprintf("/%s/", common.ToCategoryPath(u)),
		urls:    cmap.New(),
		cli:     cli,
		watcher: watcher,
	}

	if err := w.init(ctx); err != nil {
		return nil, err
	}

	ctx1, cancelFunc := context.WithCancel(context.Background())
	w.cancelFunc = cancelFunc
	go w.loop(ctx1)
	return w, nil
}

func (w *subscribeWatcher) init(ctx context.Context) error {

	resp, err := w.cli.Get(ctx, w.path, v3.WithPrefix())
	if err != nil {
		return err
	}

	for _, kv := range resp.Kvs {
		w.storeToUrls(string(kv.Value), true)
	}
	w.notify()
	return nil
}

func (w *subscribeWatcher) loop(ctx context.Context) {

	watchChan := w.watcher.Watch(ctx, w.path, v3.WithPrefix())

	for {

		select {
		case <-ctx.Done():
			return
		case ch := <-watchChan:
			for _, event := range ch.Events {
				w.handleEvent(event)
			}

		}
	}

}

func (w *subscribeWatcher) handleEvent(event *v3.Event) {
	w.storeToUrls(string(event.Kv.Value), event.IsCreate() || event.IsModify())
	w.notify()
}

func (w *subscribeWatcher) storeToUrls(raw string, updated bool) {

	u, err := common.Decode(string(raw))
	if err != nil {
		log.Errorf("failed to the url for %s decode,cause: %s", raw, err.Error())
		return
	}

	if updated {
		w.urls.Set(u.String(), u)
	} else {
		w.urls.Remove(u.String())
	}
}

func (w *subscribeWatcher) notify() {

	items := w.urls.Items()
	urls := make([]common.URL, 0)
	for _, item := range items {
		urls = append(urls, item.(common.URL))
	}
	log.Infof("notify urls for %+v", urls)
	w.l(urls)
}

func (w *subscribeWatcher) Close() error {
	w.cancelFunc()
	return nil
}
