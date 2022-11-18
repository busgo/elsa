package discovery

import (
	"fmt"

	"github.com/busgo/elsa/pkg/common"
	"github.com/busgo/elsa/registry"
	"google.golang.org/grpc/resolver"
)

type DiscoveryService interface {
	resolver.Builder
}

type discoveryService struct {
	scheme          string
	registryService registry.RegistryService
}

func New(scheme string, registryService registry.RegistryService) DiscoveryService {

	b := resolver.Get(scheme)
	if b != nil {
		return b
	}

	srv := &discoveryService{
		scheme:          scheme,
		registryService: registryService,
	}

	resolver.Register(srv)
	return srv
}

// Build creates a new resolver for the given target.
//
// gRPC dial calls Build synchronously, and fails if the returned error is
// not nil.
func (srv *discoveryService) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {

	switch srv.scheme {
	case common.DefaultProtocol:
		r, err := newElsaResolver(target, cc, srv.registryService)
		if err != nil {
			return nil, err
		}
		r.ResolveNow(resolver.ResolveNowOptions{})
		return r, nil
	default:
		return newDirectResolver(target, cc), nil
	}

}

// Scheme returns the scheme supported by this resolver.
// Scheme is defined at https://github.com/grpc/grpc/blob/master/doc/naming.md.
func (srv *discoveryService) Scheme() string {
	return srv.scheme
}

type elsaResolver struct {
	registryService registry.RegistryService
	cc              resolver.ClientConn
	u               common.URL
	urls            []common.URL
}

func newElsaResolver(target resolver.Target, cc resolver.ClientConn, registryService registry.RegistryService) (resolver.Resolver, error) {

	scheme := target.Scheme
	host := target.Authority
	endpoint := target.Endpoint

	raw := fmt.Sprintf("%s://%s/%s", scheme, host, endpoint)

	u, err := common.Decode(raw)
	if err != nil {
		return nil, err
	}
	u.Parameters["category"] = common.DefaultCategory

	r := &elsaResolver{
		u:               u,
		urls:            make([]common.URL, 0),
		cc:              cc,
		registryService: registryService,
	}
	r.registryService.Subscribe(u, func(us []common.URL) {
		r.urls = us
		r.ResolveNow(resolver.ResolveNowOptions{})
	})

	return r, nil
}

// ResolveNow will be called by gRPC to try to resolve the target name
// again. It's just a hint, resolver can ignore this if it's not necessary.
//
// It could be called multiple times concurrently.
func (r *elsaResolver) ResolveNow(resolver.ResolveNowOptions) {

	addresses := make([]resolver.Address, 0)
	for _, u := range r.urls {
		addresses = append(addresses, resolver.Address{
			Addr: u.Host,
		})
	}
	r.cc.UpdateState(resolver.State{
		Addresses: addresses,
	})
}

// Close closes the resolver.
func (r *elsaResolver) Close() {
	r.registryService.Unregister(r.u)
}

type directResolver struct {
	target resolver.Target
	cc     resolver.ClientConn
}

func newDirectResolver(target resolver.Target, cc resolver.ClientConn) resolver.Resolver {

	return &directResolver{
		target: target,
		cc:     cc,
	}
}

// ResolveNow will be called by gRPC to try to resolve the target name
// again. It's just a hint, resolver can ignore this if it's not necessary.
//
// It could be called multiple times concurrently.
func (d *directResolver) ResolveNow(resolver.ResolveNowOptions) {
	addresses := make([]resolver.Address, 0)

	addresses = append(addresses, resolver.Address{
		Addr: d.target.Authority,
	})
	d.cc.UpdateState(resolver.State{
		Addresses: addresses,
	})
}

// Close closes the resolver.
func (d *directResolver) Close() {

}
