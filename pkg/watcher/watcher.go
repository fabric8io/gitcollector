package watcher

import (
	"github.com/fabric8io/gitcollector/pkg/util"
	oclient "github.com/openshift/origin/pkg/client"
	kapi "k8s.io/kubernetes/pkg/api"
	k8sclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/watch"
)

type BuildWatcher struct {
	kubeClient *k8sclient.Client
	osClient   *oclient.Client
	watch      *watch.Interface

	namespace string
}

func New(c *k8sclient.Client, oc *oclient.Client, ns string) BuildWatcher {
	return BuildWatcher{
		kubeClient: c,
		osClient:   oc,
		namespace:  ns,
	}
}

func (b *BuildWatcher) start() {
	ns := b.namespace
	opts := kapi.ListOptions{}
	w, err := b.osClient.BuildConfigs(ns).Watch(opts)
	if err != nil {
		return err
	}
	b.watch = w

}

func (b *BuildWatcher) Run() error {
	w := b.watch
	for {
		got, ok := <-w.ResultChan()
		if ok {
			util.Infof("Found result %v\n", got)

		}
	}
	return nil
}
