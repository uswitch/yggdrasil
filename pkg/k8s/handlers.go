package k8s

import (
	"context"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"
)

func (a *Aggregator) EventsIngresses(ctx context.Context, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				a.events <- SyncDataEvent{SyncType: INGRESS}
				logrus.Debugf("adding %+v", obj)
			},
			DeleteFunc: func(obj interface{}) {
				a.events <- SyncDataEvent{SyncType: INGRESS}
				logrus.Debugf("deleting %+v", obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				a.events <- SyncDataEvent{SyncType: INGRESS}
				logrus.Debugf("updating %+v", newObj)
			},
		},
	)
	go informer.Run(ctx.Done())
}

func (a *Aggregator) EventsSecrets(ctx context.Context, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				a.events <- SyncDataEvent{SyncType: SECRET}
				logrus.Debugf("adding %+v", obj)
			},
			DeleteFunc: func(obj interface{}) {
				a.events <- SyncDataEvent{SyncType: SECRET}
				logrus.Debugf("deleting %+v", obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				a.events <- SyncDataEvent{SyncType: SECRET}
				logrus.Debugf("updating %+v", newObj)
			},
		},
	)
	go informer.Run(ctx.Done())
}
