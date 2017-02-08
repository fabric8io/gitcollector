package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fabric8io/gitcollector/pkg/publisher"
	"github.com/fabric8io/gitcollector/pkg/util"
	buildapi "github.com/openshift/origin/pkg/build/api"
	oclient "github.com/openshift/origin/pkg/client"
	kapi "k8s.io/kubernetes/pkg/api"
	k8sclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/watch"
)

const (
	noProjectSleepDelay  = 1 * time.Second
	afterEventSleepDelay = 1 * time.Second

	externalGitUri = "fabric8.io/git-clone-url"

	useGithub = false
	useGoGit  = false
)

type WatchFlags struct {
	WorkDir        string
	Namespace      string
	ExternalGitUrl bool
}

type Watcher struct {
	kubeClient *k8sclient.Client
	osClient   *oclient.Client
	publisher  publisher.Publisher
	watch      *watch.Interface
	flags      *WatchFlags

	namespace       string
	workDir         string
	currentPosition int
	collectors      []*BuildConfigCollector
}

func New(c *k8sclient.Client, oc *oclient.Client, flags *WatchFlags) Watcher {
	pub := publisher.New()
	workDir := flags.WorkDir
	err := os.MkdirAll(workDir, 0700)
	if err != nil {
		util.Errorf("Unable to create work directory %s due to: %v\n", workDir, err)
	}
	return Watcher{
		kubeClient:      c,
		osClient:        oc,
		publisher:       pub,
		flags:           flags,
		namespace:       flags.Namespace,
		workDir:         workDir,
		collectors:      []*BuildConfigCollector{},
		currentPosition: -1,
	}
}

func (b *Watcher) Run(stopCh <-chan struct{}) error {
	ns := b.namespace
	opts := kapi.ListOptions{}
	oc := b.osClient

	bcl, err := oc.BuildConfigs(ns).List(opts)
	if err != nil {
		return fmt.Errorf("Failed to find BuildConfig resources in namespace %s due to %v", ns, err)
	}
	util.Infof("Found %d BuildConfigs\n", len(bcl.Items))
	for _, bc := range bcl.Items {
		b.addBuildConfig(&bc)
	}

	w, err := oc.BuildConfigs(ns).Watch(opts)
	if err != nil {
		return fmt.Errorf("Failed to watch BuildConfig resources in namespace %s due to %v", ns, err)
	}
	b.watch = &w
	watchCh := w.ResultChan()
	for {
		select {
		// check if we're shutdown
		case <-stopCh:
			return nil

		case got, ok := <-watchCh:
			if ok {
				bc, isBC := got.Object.(*buildapi.BuildConfig)
				if !isBC || bc == nil {
					util.Warnf("received unknown object while watching for BuildConfig: %v\n", got.Object)
				} else {
					switch got.Type {
					case watch.Added:
						b.addBuildConfig(bc)
					case watch.Modified:
						b.modifyBuildConfig(bc)
					case watch.Deleted:
						b.deleteBuildConfig(bc)
					}

				}
			}

		default:
			// TODO should we sleep so we don't DOS the back end? :)
			b.processNextBuildConfig()
		}
	}
	<-stopCh
	return nil
}

func (b *Watcher) processNextBuildConfig() {
	size := len(b.collectors)
	if size == 0 {
		time.Sleep(noProjectSleepDelay)
		return
	}
	pos := b.currentPosition + 1
	if pos >= size {
		pos = 0
	}
	b.currentPosition = pos
	buildWatch := b.collectors[pos]
	if buildWatch.Process() > 0 {
		time.Sleep(afterEventSleepDelay)
	}
}

func (b *Watcher) addBuildConfig(bc *buildapi.BuildConfig) {
	b.upsertBuildConfig(bc, true)
}

func (b *Watcher) modifyBuildConfig(bc *buildapi.BuildConfig) {
	b.upsertBuildConfig(bc, false)
}

func (b *Watcher) upsertBuildConfig(bc *buildapi.BuildConfig, add bool) {
	message := "updating"
	if add {
		message = "adding"
	}
	name := bc.Name
	newGS := b.GitSource(bc)
	if newGS == nil {
		return
	}
	util.Infof("%s BuildConfig %s with source %v\n", message, name, newGS)
	var buildWatch *BuildConfigCollector = nil
	for _, bw := range b.collectors {
		if name == bw.name {
			buildWatch = bw
			break
		}
	}
	var oldBc *buildapi.BuildConfig = nil
	if buildWatch == nil {
		buildWatch = &BuildConfigCollector{
			name:        name,
			watcher:     b,
			buildConfig: *bc,
			workDir:     filepath.Join(b.workDir, name),
		}
		b.collectors = append(b.collectors, buildWatch)
	} else {
		oldBc = &buildWatch.buildConfig
		buildWatch.buildConfig = *bc
	}

	if oldBc != nil {
		oldGS := b.GitSource(oldBc)
		if removeOldGitSource(oldGS, newGS) {
			// the git branch/repo has changed so lets remove the data
			util.Infof("Git source changed for %s so lets remove old files as its %v and was %v\n", name, newGS, oldGS)
			buildWatch.Delete()
		}

	}

	b.publisher.UpsertBuildConfig(bc)
}

func (b *Watcher) deleteBuildConfig(bc *buildapi.BuildConfig) {
	name := bc.Name
	util.Infof("removing BuildConfig %s\n", name)
	for i, bw := range b.collectors {
		if name == bw.name {
			bw.Delete()
			s := b.collectors
			b.collectors = append(s[:i], s[i+1:]...)

			// lets avoid missing the next item when we process things
			if b.currentPosition >= i {
				b.currentPosition -= 1
			}
			break
		}
	}
}

func (b *Watcher) GitSource(bc *buildapi.BuildConfig) *buildapi.GitBuildSource {
	if bc != nil {
		gs := bc.Spec.Source.Git
		if gs == nil {
			return nil
		}
		if b.flags.ExternalGitUrl {
			ann := bc.Annotations
			if ann != nil {
				uri := ann[externalGitUri]
				if len(uri) > 0 {
					return &buildapi.GitBuildSource{
						URI:         uri,
						Ref:         gs.Ref,
						ProxyConfig: gs.ProxyConfig,
					}
				}
			}
		}
		return gs
	}
	return nil
}

func removeOldGitSource(old *buildapi.GitBuildSource, new *buildapi.GitBuildSource) bool {
	if old == new || old == nil || len(old.URI) == 0 {
		return false
	}
	if new == nil {
		return len(old.URI) > 0
	}
	return old.URI != new.URI || old.Ref != new.Ref
}
