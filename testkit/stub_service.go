package testkit

import (
	"context"
	"sync"

	"github.com/dpopsuev/battery/service"
)

// StubService implements service.Service for testing.
// Records Start/Stop calls and returns configurable health.
type StubService struct {
	mu      sync.Mutex
	name    string
	health  service.HealthReport
	Started bool
	Stopped bool
}

var _ service.Service = (*StubService)(nil)

func NewStubService(name string) *StubService {
	return &StubService{
		name:   name,
		health: service.HealthReport{Status: service.Healthy},
	}
}

func (s *StubService) Name() string { return s.name }

func (s *StubService) Start(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Started = true
	return nil
}

func (s *StubService) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Stopped = true
	return nil
}

func (s *StubService) Health() service.HealthReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.health
}

// SetHealth configures the health report for testing.
func (s *StubService) SetHealth(h service.HealthReport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.health = h
}

// StubObserver implements service.Observer for testing.
// Returns configurable alerts and records Check calls.
type StubObserver struct {
	StubService
	category string
	alerts   []service.Alert
	Checks   int
}

var _ service.Observer = (*StubObserver)(nil)

func NewStubObserver(name, category string) *StubObserver {
	return &StubObserver{
		StubService: *NewStubService(name),
		category:    category,
	}
}

func (o *StubObserver) Check(_ context.Context) []service.Alert {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.Checks++
	return o.alerts
}

func (o *StubObserver) Category() string { return o.category }

// SetAlerts configures the alerts returned by Check.
func (o *StubObserver) SetAlerts(alerts ...service.Alert) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.alerts = alerts
}

// StubController implements service.Controller for testing.
// Records Handle calls and returns configurable error.
type StubController struct {
	StubService
	subjects []string
	handleFn func(context.Context, service.Alert) error
	Handled  []service.Alert
}

var _ service.Controller = (*StubController)(nil)

func NewStubController(name string, subjects ...string) *StubController {
	return &StubController{
		StubService: *NewStubService(name),
		subjects:    subjects,
	}
}

func (c *StubController) Handle(ctx context.Context, alert service.Alert) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Handled = append(c.Handled, alert)
	if c.handleFn != nil {
		return c.handleFn(ctx, alert)
	}
	return nil
}

func (c *StubController) Subjects() []string { return c.subjects }

// SetHandleFn configures a custom handle function for testing.
func (c *StubController) SetHandleFn(fn func(context.Context, service.Alert) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handleFn = fn
}
