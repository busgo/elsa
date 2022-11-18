package least

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/grpclog"
)

// Name is the name of round_robin balancer.
const Name = "least_connection"

var logger = grpclog.Component("least_connection")

// newBuilder creates a new roundrobin balancer builder.
func newBuilder() balancer.Builder {
	return base.NewBalancerBuilder(Name, &rrPickerBuilder{}, base.Config{HealthCheck: true})
}

func init() {
	balancer.Register(newBuilder())
}

type rrPickerBuilder struct{}

func (*rrPickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	logger.Infof("leastConnectionPicker: Build called with info: %v", info)
	if len(info.ReadySCs) == 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}
	scs := make([]*node, 0, len(info.ReadySCs))
	for sc := range info.ReadySCs {
		scs = append(scs, &node{
			subConn:  sc,
			inflight: 0,
		})
	}
	return &rrPicker{
		nodes: scs,
		rand:  rand.New(rand.NewSource(time.Now().Unix())),
		l:     sync.Mutex{},
	}
}

type rrPicker struct {
	nodes []*node
	rand  *rand.Rand
	l     sync.Mutex
}

type node struct {
	subConn  balancer.SubConn
	inflight int64
}

// Pick returns the connection to use for this RPC and related information.
//
// Pick should not block.  If the balancer needs to do I/O or any blocking
// or time-consuming work to service this call, it should return
// ErrNoSubConnAvailable, and the Pick call will be repeated by gRPC when
// the Picker is updated (using ClientConn.UpdateState).
//
// If an error is returned:
//
// - If the error is ErrNoSubConnAvailable, gRPC will block until a new
//   Picker is provided by the balancer (using ClientConn.UpdateState).
//
// - If the error is a status error (implemented by the grpc/status
//   package), gRPC will terminate the RPC with the code and message
//   provided.
//
// - For all other errors, wait for ready RPCs will wait, but non-wait for
//   ready RPCs will be terminated with this error's Error() string and
//   status code Unavailable.
func (p *rrPicker) Pick(balancer.PickInfo) (balancer.PickResult, error) {

	l := len(p.nodes)
	if l == 0 {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	if l == 1 {
		return balancer.PickResult{SubConn: p.nodes[0].subConn}, nil
	}

	p.l.Lock()

	a := p.rand.Intn(l)
	b := p.rand.Intn(l)
	p.l.Unlock()

	if a == b {
		b = (b + 1) % l
	}

	var n *node

	if p.nodes[a].inflight <= p.nodes[b].inflight {
		n = p.nodes[a]
	} else {
		n = p.nodes[b]
	}

	atomic.AddInt64(&n.inflight, 1)

	ret := balancer.PickResult{
		SubConn: n.subConn,
		Done: func(di balancer.DoneInfo) {
			atomic.AddInt64(&n.inflight, -1)
		},
	}

	return ret, nil

}
