package service_test

import (
	"context"
	"testing"

	"github.com/dpopsuev/battery/service"
	"github.com/dpopsuev/battery/testkit"
)

func TestStubService_Lifecycle(t *testing.T) {
	s := testkit.NewStubService("test-svc")

	if s.Name() != "test-svc" {
		t.Fatalf("Name = %q, want test-svc", s.Name())
	}

	h := s.Health()
	if h.Status != service.Healthy {
		t.Fatalf("Health = %q, want healthy", h.Status)
	}

	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !s.Started {
		t.Fatal("Started should be true")
	}

	if err := s.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !s.Stopped {
		t.Fatal("Stopped should be true")
	}
}

func TestStubObserver_Check(t *testing.T) {
	o := testkit.NewStubObserver("budget-wd", "budget")

	if o.Category() != "budget" {
		t.Fatalf("Category = %q, want budget", o.Category())
	}

	// No alerts configured.
	alerts := o.Check(context.Background())
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts, got %d", len(alerts))
	}

	// Configure alerts.
	o.SetAlerts(service.Alert{
		Category: "budget",
		Level:    "warning",
		Message:  "budget at 92%",
	})

	alerts = o.Check(context.Background())
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Message != "budget at 92%" {
		t.Fatalf("Message = %q", alerts[0].Message)
	}
	if o.Checks != 2 {
		t.Fatalf("Checks = %d, want 2", o.Checks)
	}
}

func TestStubController_Handle(t *testing.T) {
	c := testkit.NewStubController("relay-mgr", "budget", "context")

	if len(c.Subjects()) != 2 {
		t.Fatalf("Subjects = %d, want 2", len(c.Subjects()))
	}

	alert := service.Alert{
		Category: "budget",
		Level:    "warning",
		Message:  "relay needed",
	}

	if err := c.Handle(context.Background(), alert); err != nil {
		t.Fatal(err)
	}
	if len(c.Handled) != 1 {
		t.Fatalf("Handled = %d, want 1", len(c.Handled))
	}
	if c.Handled[0].Message != "relay needed" {
		t.Fatalf("Handled[0].Message = %q", c.Handled[0].Message)
	}
}

func TestObserver_ImplementsService(t *testing.T) {
	var _ service.Service = testkit.NewStubObserver("x", "x")
}

func TestController_ImplementsService(t *testing.T) {
	var _ service.Service = testkit.NewStubController("x")
}
