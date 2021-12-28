package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
)

type IngressLister interface {
	List() ([]v1beta1.Ingress, error)
}

//IngressAggregator used for running Ingress infomers
type IngressAggregator struct {
	stores      []cache.Store
	controllers []cache.Controller
	events      chan interface{}
}

func (i *IngressAggregator) Events() chan interface{} {
	return i.events
}

//Run starts all the ingress informers. Will block until all controllers
//have synced. Shouldn't be called in go routine
func (i *IngressAggregator) Run(ctx context.Context) error {
	for _, c := range i.controllers {
		logrus.Debugf("starting cache controller: %+v", c)
		go c.Run(ctx.Done())
		cache.WaitForCacheSync(ctx.Done(), c.HasSynced)
		logrus.Debugf("cache controller synced")
	}
	return nil
}

func (i *IngressAggregator) OnAdd(obj interface{}) {
	i.events <- obj
	logrus.Debugf("adding %+v", obj)
}

func (i *IngressAggregator) OnDelete(obj interface{}) {
	i.events <- obj
	logrus.Debugf("deleting %+v", obj)
}

func (i *IngressAggregator) OnUpdate(old, new interface{}) {
	i.events <- new
	logrus.Debugf("updating %+v", new)
}

//AddSource adds a new source for watching ingresses, must be called before running
func (i *IngressAggregator) AddSource(source cache.ListerWatcher) {
	//Todo implement handler for events
	store, controller := cache.NewIndexerInformer(source, &v1beta1.Ingress{}, time.Minute, i, cache.Indexers{})
	i.stores = append(i.stores, store)
	i.controllers = append(i.controllers, controller)
}

//NewIngressAggregator returns a new ingress IngressAggregator
func NewIngressAggregator(sources []cache.ListerWatcher) *IngressAggregator {
	a := &IngressAggregator{
		events: make(chan interface{}),
	}
	for _, s := range sources {
		a.AddSource(s)
	}
	return a
}

//List returns all ingresses
func (i *IngressAggregator) List() ([]v1beta1.Ingress, error) {
	is := make([]v1beta1.Ingress, 0)
	for _, store := range i.stores {
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
