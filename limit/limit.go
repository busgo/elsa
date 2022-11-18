package limit

import "fmt"

var (
	_limiters = make(map[string]Limiter)
)

func Use(name string, limiter Limiter) {
	_limiters[fmt.Sprintf("elsa.grpc.limiter.%s", name)] = limiter
}

func Load(name string) Limiter {
	return _limiters[fmt.Sprintf("elsa.grpc.limiter.%s", name)]
}

type Limiter interface {
	Take() bool
	Return()
}

type limiter struct {
	concurrency int64

	tickets chan struct{}
}

func New(concurrency int64) Limiter {

	return &limiter{
		concurrency: concurrency,
		tickets:     make(chan struct{}, concurrency),
	}
}

func (l *limiter) Take() bool {
	if l.concurrency <= 0 || l.tickets == nil {
		return true
	}

	select {
	case l.tickets <- struct{}{}:
		return true
	default:
		return false
	}
}
func (l *limiter) Return() {

	if l.concurrency <= 0 || l.tickets == nil {
		return
	}

	select {
	case <-l.tickets:
		return
	default:
		return

	}

}
