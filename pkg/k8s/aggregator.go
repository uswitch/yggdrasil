package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type IngressLister interface {
	List() ([]v1beta1.Ingress, error)
}

type Aggregator struct {
	factories     []*informers.SharedInformerFactory
	events        chan SyncDataEvent
	ingressStores []cache.Store
}

func (a *Aggregator) Events() chan SyncDataEvent {
	return a.events
}

// NewAggregator returns a new Aggregator initialized with resource informers
func NewAggregator(k8sClients []*kubernetes.Clientset, ctx context.Context) *Aggregator {
	a := Aggregator{
		events:        make(chan SyncDataEvent, watch.DefaultChanSize),
		ingressStores: []cache.Store{},
	}
	informersSynced := []cache.InformerSynced{}

	for _, c := range k8sClients {
		factory := informers.NewSharedInformerFactory(c, time.Minute)

		ingressInformer := factory.Extensions().V1beta1().Ingresses().Informer()
		a.EventsIngresses(ctx, ingressInformer)
		a.ingressStores = append(a.ingressStores, ingressInformer.GetStore())

		a.factories = append(a.factories, &factory)
		informersSynced = append(informersSynced, ingressInformer.HasSynced)
	}

	if !cache.WaitForCacheSync(ctx.Done(), informersSynced...) {
		logrus.Panicf("Unable to populate caches")
	}
	return &a
}

// Run is the synchronization loop
func (a *Aggregator) Run() {
	for {
		time.Sleep(5 * time.Second)
		a.events <- SyncDataEvent{SyncType: COMMAND}
	}
}

// ListIngresses returns all ingresses
func (a *Aggregator) ListIngresses() ([]v1beta1.Ingress, error) {
	is := make([]v1beta1.Ingress, 0)
	for _, store := range a.ingressStores {
		ingresses := store.List()
		for _, obj := range ingresses {
			ingress, ok := obj.(*v1beta1.Ingress)
			if !ok {
				return nil, fmt.Errorf("unexpected object in store: %+v", obj)
			}
			is = append(is, *ingress)
		}
	}
	return is, nil
}
