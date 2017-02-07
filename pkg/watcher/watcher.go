package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/fabric8io/gitcollector/pkg/util"
	"github.com/google/go-github/github"
	"github.com/src-d/go-git"
	"k8s.io/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
	oclient "github.com/openshift/origin/pkg/client"
	kapi "k8s.io/kubernetes/pkg/api"
	k8sclient "k8s.io/kubernetes/pkg/client/unversioned"
)

const (
	noProjectSleepDelay  = 1 * time.Second
	afterEventSleepDelay = 1 * time.Second

	externalGitUri = "fabric8.io/git-clone-url"
)

type WatchFlags struct {
	WorkDir        string
	Namespace      string
	ExternalGitUrl bool
}

type BuildConfigWatch struct {
	watcher     *Watcher
	buildConfig *buildapi.BuildConfig
	workDir     string
}

type Watcher struct {
	kubeClient *k8sclient.Client
	osClient   *oclient.Client
	watch      *watch.Interface
	flags      *WatchFlags

	namespace              string
	workDir                string
	currentBuildConfigName string
	currentPosition        int
	buildConfigs           map[string]*BuildConfigWatch
	buildConfigNames       []string
}

func New(c *k8sclient.Client, oc *oclient.Client, flags *WatchFlags) Watcher {
	workDir := flags.WorkDir
	err := os.MkdirAll(workDir, 0700)
	if err != nil {
		util.Errorf("Unable to create work directory %s due to: %v\n", workDir, err)
	}
	return Watcher{
		kubeClient:       c,
		osClient:         oc,
		flags:            flags,
		namespace:        flags.Namespace,
		workDir:          workDir,
		buildConfigs:     map[string]*BuildConfigWatch{},
		buildConfigNames: []string{},
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
	name, pos := nextName(b.currentBuildConfigName, b.currentPosition, b.buildConfigNames)
	b.currentBuildConfigName = name
	b.currentPosition = pos
	if len(name) == 0 {
		time.Sleep(noProjectSleepDelay)
		return
	}
	buildWatch := b.buildConfigs[name]
	if buildWatch == nil {
		util.Warnf("No BuildWatch exists for %s\n", name)
		return
	}
	if buildWatch.Process() > 0 {
		time.Sleep(afterEventSleepDelay)
	}
}

// nextName returns the next name in order given the current name
// if no name selected then return the first otherwise return the next one
// in the list, rolling around to the first one again
func nextName(current string, pos int, names []string) (string, int) {
	if len(names) == 0 {
		return "", 0
	}
	if len(current) == 0 {
		return names[0], 0
	}
	size := len(names)
	for i := 0; i < size; i++ {
		name := names[i]
		if current == name {
			j := i + 1
			if j >= size {
				return names[0], 0
			} else {
				return names[j], j
			}
		}
	}

	// the current item was removed so lets assume the previous position to avoid
	// weighting the early names too much
	if pos < size {
		return names[pos], pos
	} else {
		return names[0], 0
	}
}

func (b *Watcher) addBuildConfig(bc *buildapi.BuildConfig) {
	util.Infof("adding BuildConfig %s\n", bc.Name)
	b.upsertBuildConfig(bc, true)
}

func (b *Watcher) modifyBuildConfig(bc *buildapi.BuildConfig) {
	util.Infof("updating BuildConfig %s\n", bc.Name)
	b.upsertBuildConfig(bc, false)
}

func (b *Watcher) upsertBuildConfig(bc *buildapi.BuildConfig, add bool) {
	name := bc.Name
	buildWatch := b.buildConfigs[name]
	var oldBc *buildapi.BuildConfig
	if buildWatch == nil {
		workDir := filepath.Join(b.workDir, name)
		err := os.MkdirAll(workDir, 0700)
		if err != nil {
			util.Errorf("Unable to create work directory %s due to: %v\n", workDir, err)
			return
		}
		buildWatch = &BuildConfigWatch{
			watcher:     b,
			buildConfig: bc,
			workDir:     workDir,
		}
		b.buildConfigs[bc.Name] = buildWatch
	} else {
		oldBc = buildWatch.buildConfig
	}
	buildWatch.buildConfig = bc

	if oldBc != nil {
		if removeOldGitSource(b.GitSource(oldBc), b.GitSource(bc)) {
			// the git branch/repo has changed so lets remove the data
			util.Infof("Git source changed for %s so lets remove old files\n", name)
			buildWatch.Delete()
		}

	}
	b.updateIndices()

	// TODO lets POST to the tracker
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

func removeOldGitSource(old, new *buildapi.GitBuildSource) bool {
	if old == new || old == nil {
		return false
	}
	if new == nil {
		return true
	}
	return old.URI != new.URI || old.Ref != new.Ref
}

func (b *Watcher) deleteBuildConfig(bc *buildapi.BuildConfig) {
	name := bc.Name
	util.Infof("removing BuildConfig %s\n", name)
	buildWatch := b.buildConfigs[name]
	if buildWatch != nil {
		delete(b.buildConfigs, name)
		buildWatch.Delete()
		b.updateIndices()
	}
}

func (b *Watcher) updateIndices() {
	names := []string{}
	for name, _ := range b.buildConfigs {
		names = append(names, name)
	}
	sort.Strings(names)
	b.buildConfigNames = names
}

// Delete removes the work directory for the given watch
func (w *BuildConfigWatch) Delete() {
	name := w.buildConfig.Name
	workDir := os.RemoveAll(w.workDir)
	err := workDir
	if err != nil {
		util.Warnf("Failed to remove workDir %s for BuildConfig %s due to: %v\n", workDir, name, err)
	}
}

func (w *BuildConfigWatch) Process() int {
	bc := w.buildConfig
	gs := w.watcher.GitSource(bc)
	if gs == nil {
		return 0
	}
	gitDir := filepath.Join(w.workDir, ".git")
	if stat, err := os.Stat(gitDir); err != nil || !stat.IsDir() {
		w.cloneRepo(gs)
	} else {
		w.pullRepo(gs)
	}

	client := github.NewClient(nil)
	util.Infof("Created new github client %v\n", client)

	//util.Infof("Processing BuildConfig %s\n", name)
	return 1
}

func (w *BuildConfigWatch) pullRepo(gs *buildapi.GitBuildSource) error {
	return nil
}

func (w *BuildConfigWatch) cloneRepo(gs *buildapi.GitBuildSource) error {
	// TODO should we check the uri & ref with the .git/config in case we restart and have a PV?

	bc := w.buildConfig
	name := bc.Name

	uri := gs.URI
	if len(uri) == 0 {
		return nil
	}
	ref := gs.Ref
	if len(ref) == 0 {
		ref = "master"
	}
	workDir := w.workDir
	util.Infof("Cloning repo %s ref %s for BuildConfig %s to \n", uri, ref, name, workDir)

	_, err := git.PlainClone(workDir, false, &git.CloneOptions{
		URL:      uri,
		Progress: os.Stdout,
	})
	return err
}
