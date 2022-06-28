package k8s

import (
	"context"
	"testing"

	v1 "k8s.io/api/networking/v1"
	kt "k8s.io/client-go/tools/cache/testing"
)

func TestListReturnsEmptyWithNoObjects(t *testing.T) {
	source := kt.NewFakeControllerSource()
	watchlist := Ingresswatcher{Watcher: source, IngressEndpoints: []string{"127.0.0.1"}}
	a := NewIngressAggregator([]Ingresswatcher{watchlist})
	go reader(context.Background(), a.Events())
	a.Run(context.Background())

	ingresses, _ := a.List()
	if len(ingresses) != 0 {
		t.Errorf("expected 0 ingresses, was %d", len(ingresses))
	}
}

func TestReturnsIngresses(t *testing.T) {
	source := kt.NewFakeControllerSource()
	source.Add(&v1.Ingress{})
	watchlist := Ingresswatcher{Watcher: source, IngressEndpoints: []string{"127.0.0.1"}}
	a := NewIngressAggregator([]Ingresswatcher{watchlist})
	go reader(context.Background(), a.Events())
	a.Run(context.Background())

	ingresses, err := a.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(ingresses) != 1 {
		t.Errorf("expected 1 ingress, found %d", len(ingresses))
	}
}

func TestReturnsFromMultipleIngressControllers(t *testing.T) {
	source1 := kt.NewFakeControllerSource()
	source1.Add(&v1.Ingress{})
	source2 := kt.NewFakeControllerSource()
	source2.Add(&v1.Ingress{})
	watchlist1 := Ingresswatcher{Watcher: source1, IngressEndpoints: []string{"127.0.0.1"}}
	watchlist2 := Ingresswatcher{Watcher: source2, IngressEndpoints: []string{"192.168.1.1"}}
	a := NewIngressAggregator([]Ingresswatcher{watchlist1, watchlist2})
	go reader(context.Background(), a.Events())
	a.Run(context.Background())

	ingresses, err := a.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(ingresses) != 2 {
		t.Errorf("expected 2 ingress, found %d", len(ingresses))
	}
}

//Need something to read from the channel
func reader(ctx context.Context, events chan interface{}) {
	go func() {
		for {
			select {
			case <-events:
			case <-ctx.Done():
				return
			}
		}
	}()
}
