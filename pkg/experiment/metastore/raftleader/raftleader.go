package raftleader

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/hashicorp/raft"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/grafana/pyroscope/pkg/util/health"
)

type HealthObserver struct {
	server     health.Service
	logger     log.Logger
	mu         sync.Mutex
	registered map[serviceKey]*raftService
	metrics    *Metrics
}
type Metrics struct {
	status prometheus.Gauge
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		status: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "pyroscope",
			Name:      "metastore_raft_status",
		}),
	}
	if reg != nil {
		reg.MustRegister(m.status)
	}
	return m
}

func NewRaftLeaderHealthObserver(hs health.Service, logger log.Logger, m *Metrics) *HealthObserver {
	return &HealthObserver{
		server:     hs,
		logger:     logger,
		metrics:    m,
		registered: make(map[serviceKey]*raftService),
	}
}

func (hs *HealthObserver) Register(r *raft.Raft, service string) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	k := serviceKey{raft: r, service: service}
	if _, ok := hs.registered[k]; ok {
		return
	}
	svc := &raftService{
		server:  hs.server,
		hs:      hs,
		logger:  log.With(hs.logger, "service", service),
		service: service,
		raft:    r,
		c:       make(chan raft.Observation, 1),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	_ = level.Debug(svc.logger).Log("msg", "registering health check")
	svc.updateStatus()
	go svc.run()
	svc.observer = raft.NewObserver(svc.c, true, func(o *raft.Observation) bool {
		_, ok := o.Data.(raft.LeaderObservation)
		return ok
	})
	r.RegisterObserver(svc.observer)
	hs.registered[k] = svc
}

func (hs *HealthObserver) Deregister(r *raft.Raft, service string) {
	hs.mu.Lock()
	k := serviceKey{raft: r, service: service}
	svc, ok := hs.registered[k]
	delete(hs.registered, k)
	hs.mu.Unlock()
	if ok {
		close(svc.stop)
		<-svc.done
	}
}

type serviceKey struct {
	raft    *raft.Raft
	service string
}

type raftService struct {
	server   health.Service
	hs       *HealthObserver
	logger   log.Logger
	service  string
	raft     *raft.Raft
	observer *raft.Observer
	c        chan raft.Observation
	stop     chan struct{}
	done     chan struct{}
}

func (svc *raftService) run() {
	defer func() {
		close(svc.done)
	}()
	for {
		select {
		case <-svc.c:
			svc.updateStatus()
		case <-svc.stop:
			_ = level.Debug(svc.logger).Log("msg", "deregistering health check")
			// We explicitly remove the service from serving when we stop observing it.
			svc.server.SetServingStatus(svc.service, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
			svc.raft.DeregisterObserver(svc.observer)
			return
		}
	}
}

func (svc *raftService) updateStatus() {
	status := grpc_health_v1.HealthCheckResponse_NOT_SERVING
	if svc.raft.State() == raft.Leader {
		status = grpc_health_v1.HealthCheckResponse_SERVING
	}
	svc.hs.metrics.status.Set(float64(svc.raft.State()))

	_ = level.Info(svc.logger).Log("msg", "updating health status", "status", status)
	svc.server.SetServingStatus(svc.service, status)
}
