package registry

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/busgo/elsa/pkg/cmap"
	"github.com/busgo/elsa/pkg/common"
	"github.com/busgo/elsa/pkg/log"
	"github.com/busgo/elsa/registry/transport"
)

type RegistryService interface {

	// Register  register a service
	Register(common.URL)

	// Unregister unregister a service
	Unregister(common.URL)

	// Subscribe subscribe a service
	Subscribe(common.URL, common.NotifyListener)

	// Unsubscribe unsubscribbe a service
	Unsubscribe(common.URL, common.NotifyListener)

	io.Closer
}

type registryService struct {
	transport   transport.RegistryTransport
	dialTimeout int64
	retryPeriod int64

	registeredUrls         cmap.ConcurrentMap
	failedRegisteredUrls   cmap.ConcurrentMap
	failedUnregisteredUrls cmap.ConcurrentMap

	subscribedUrls         cmap.ConcurrentMap
	failedSubscribedUrls   cmap.ConcurrentMap
	failedUnsubscribedUrls cmap.ConcurrentMap

	cancelFunc context.CancelFunc
}

func New(opts ...Option) (RegistryService, error) {

	cc := _config

	for _, opt := range opts {
		opt(&cc)
	}
	if cc.transport == nil {
		return nil, errors.New("the registry transport is nil")
	}

	srv := &registryService{
		transport:              cc.transport,
		dialTimeout:            cc.dialTimeout,
		retryPeriod:            cc.retryPeriod,
		registeredUrls:         cmap.New(),
		failedRegisteredUrls:   cmap.New(),
		failedUnregisteredUrls: cmap.New(),
		subscribedUrls:         cmap.New(),
		failedSubscribedUrls:   cmap.New(),
		failedUnsubscribedUrls: cmap.New(),
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	srv.cancelFunc = cancelFunc
	go srv.loop(ctx)
	return srv, nil
}

func (srv *registryService) loop(ctx context.Context) {

	for {

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * time.Duration(srv.retryPeriod)):
			srv.retry()
		case <-srv.transport.Recover():
			srv.recover()
		}
	}
}

func (srv *registryService) recover() {

	// recover registered url
	if srv.registeredUrls.Count() > 0 {
		items := srv.registeredUrls.Items()
		for k, v := range items {
			srv.failedRegisteredUrls.Set(k, v)
			log.Infof("recovering register url for %s, waiting for retry", k)
		}
	}
	// recover subscribe url
	if srv.subscribedUrls.Count() > 0 {
		items := srv.subscribedUrls.Items()
		for k, v := range items {
			srv.failedSubscribedUrls.Set(k, v)
			log.Infof("recovering subscribe url for %s, waiting for retry", k)
		}
	}

}

func (srv *registryService) retry() {

	// failed registered urls
	if srv.failedRegisteredUrls.Count() > 0 {
		items := srv.failedRegisteredUrls.Items()

		for key, item := range items {
			ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*time.Duration(srv.dialTimeout))
			defer cancelFunc()
			if err := srv.transport.Register(ctx, item.(common.URL)); err != nil {
				log.Warnf("retrying to register url for %s register fail, waiting for retry again,cause: %s", key, err.Error())
				continue
			}
			log.Infof("retrying to register url for %s success", key)
			srv.failedRegisteredUrls.Remove(key)
		}

	}

	// failed unregistered urls
	if srv.failedUnregisteredUrls.Count() > 0 {
		items := srv.failedUnregisteredUrls.Items()
		for key, item := range items {
			ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*time.Duration(srv.dialTimeout))
			defer cancelFunc()
			if err := srv.transport.Unregister(ctx, item.(common.URL)); err != nil {
				log.Warnf("retrying to register url for %s unregister fail, waiting for retry again,cause: %s", key, err.Error())
				continue
			}
			log.Infof("retrying to register url for %s unregister success", key)
			srv.failedUnregisteredUrls.Remove(key)
		}

	}
	// failed subscribed urls
	if srv.failedSubscribedUrls.Count() > 0 {

		items := srv.failedSubscribedUrls.Items()

		for key, v := range items {

			ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*time.Duration(srv.dialTimeout))
			defer cancelFunc()

			item := v.(subscribeItem)
			if err := srv.transport.Subscribe(ctx, item.u, item.l); err != nil {
				log.Warnf("retrying to subscribe url for %s subscribe fail, waiting for retry again,cause: %s", key, err.Error())
				continue
			}
			log.Infof("retrying to subscribe url for %s subscribe success", key)
			srv.failedSubscribedUrls.Remove(key)
		}

	}

	//

	// failed unsubscribed urls
	if srv.failedUnsubscribedUrls.Count() > 0 {

		items := srv.failedUnsubscribedUrls.Items()

		for key, v := range items {

			ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*time.Duration(srv.dialTimeout))
			defer cancelFunc()

			item := v.(subscribeItem)
			if err := srv.transport.Unsubscribe(ctx, item.u, item.l); err != nil {
				log.Warnf("retrying to subscribe url for %s unsubscribe fail, waiting for retry again,cause: %s", key, err.Error())
				continue
			}
			log.Infof("retrying to subscribe url for %s unsubscribe success", key)
			srv.failedUnsubscribedUrls.Remove(key)
		}

	}

}

func (srv *registryService) closeAll() {

	if srv.registeredUrls.Count() > 0 {
		items := srv.registeredUrls.Items()
		for _, v := range items {
			srv.Unregister(v.(common.URL))
		}
	}

	if srv.subscribedUrls.Count() > 0 {
		items := srv.subscribedUrls.Items()
		for _, v := range items {
			it := v.(subscribeItem)
			srv.Unsubscribe(it.u, it.l)
		}
	}

}

// Register  register a service
func (srv *registryService) Register(u common.URL) {
	srv.registeredUrls.Set(u.String(), u)
	srv.failedRegisteredUrls.Remove(u.String())
	srv.failedUnregisteredUrls.Remove(u.String())

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*time.Duration(srv.dialTimeout))
	defer cancelFunc()
	if err := srv.transport.Register(ctx, u); err != nil {
		log.Warnf("failed to register url for %s to register,waiting for retry, cause: %s", u.String(), err.Error())
		srv.failedRegisteredUrls.Set(u.String(), u)
		return
	}

	log.Infof("the register url for %s register success", u.String())
}

// Unregister unregister a service
func (srv *registryService) Unregister(u common.URL) {
	srv.registeredUrls.Remove(u.String())
	srv.failedRegisteredUrls.Remove(u.String())
	srv.failedUnregisteredUrls.Remove(u.String())

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*time.Duration(srv.dialTimeout))
	defer cancelFunc()
	if err := srv.transport.Unregister(ctx, u); err != nil {
		log.Warnf("failed to register for url %s to unregister, waiting for retry, cause: %s", u.String(), err.Error())
		srv.failedUnregisteredUrls.Set(u.String(), u)
		return
	}

	log.Infof("the register url for %s unregister success", u.String())
}

type subscribeItem struct {
	u common.URL
	l common.NotifyListener
}

// Subscribe subscribe a service
func (srv *registryService) Subscribe(u common.URL, l common.NotifyListener) {
	srv.subscribedUrls.Set(u.String(), subscribeItem{u: u, l: l})
	srv.failedSubscribedUrls.Remove(u.String())
	srv.failedUnsubscribedUrls.Remove(u.String())

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*time.Duration(srv.dialTimeout))
	defer cancelFunc()

	if err := srv.transport.Subscribe(ctx, u, l); err != nil {
		log.Warnf("failed to subscribe for url %s to subscribe, waiting for retry, cause: %s", u.String(), err.Error())
		srv.failedSubscribedUrls.Set(u.String(), subscribeItem{u: u, l: l})
		return
	}

	log.Infof("the subscribe url for %s to subscribe success", u.String())
}

// Unsubscribe unsubscribbe a service
func (srv *registryService) Unsubscribe(u common.URL, l common.NotifyListener) {
	srv.subscribedUrls.Remove(u.String())
	srv.failedSubscribedUrls.Remove(u.String())
	srv.failedUnsubscribedUrls.Remove(u.String())

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*time.Duration(srv.dialTimeout))
	defer cancelFunc()

	if err := srv.transport.Unsubscribe(ctx, u, l); err != nil {
		log.Warnf("failed to subscribe url for %s to unsubscribe, waiting to retry, cause: %s", u.String(), err.Error())
		srv.failedUnsubscribedUrls.Set(u.String(), subscribeItem{u: u, l: l})
		return
	}

	log.Infof("the subscribe url for %s to unsubscribe success", u.String())
}

func (srv *registryService) Close() error {
	srv.cancelFunc()
	srv.closeAll()
	return srv.transport.Close()
}
