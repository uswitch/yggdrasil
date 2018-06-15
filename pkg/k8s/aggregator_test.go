package k8s

import (
	"context"
	"testing"

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	kt "k8s.io/client-go/tools/cache/testing"
)

func TestListReturnsEmptyWithNoObjects(t *testing.T) {
	source := kt.NewFakeControllerSource()
	a := NewIngressAggregator([]cache.ListerWatcher{source})
	a.Run(context.Background())

	ingresses, _ := a.List()
	if len(ingresses) != 0 {
		t.Errorf("expected 0 ingresses, was %d", len(ingresses))
	}
}

func TestReturnsIngresses(t *testing.T) {
	source := kt.NewFakeControllerSource()
	source.Add(&v1beta1.Ingress{})

	a := NewIngressAggregator([]cache.ListerWatcher{source})
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
	source1.Add(&v1beta1.Ingress{})
	source2 := kt.NewFakeControllerSource()
	source2.Add(&v1beta1.Ingress{})

	a := NewIngressAggregator([]cache.ListerWatcher{source1, source2})
	a.Run(context.Background())

	ingresses, err := a.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(ingresses) != 2 {
		t.Errorf("expected 2 ingress, found %d", len(ingresses))
	}
}
