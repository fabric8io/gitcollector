package watcher

import (
	"fmt"
	"github.com/fabric8io/gitcollector/pkg/util"
	"github.com/google/go-github/github"
	"github.com/src-d/go-git"
	"os"
	"os/exec"
	"path/filepath"
	"srcd.works/go-git.v4/plumbing"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"io"
)

const (
	maxCommits = 10
)

type BuildConfigCollector struct {
	name        string
	workDir     string
	watcher     *Watcher
	buildConfig buildapi.BuildConfig

	firstGitHash string
	lastGitHash  string
}

// Delete removes the work directory for the given watch
func (w *BuildConfigCollector) Delete() {
	name := w.buildConfig.Name
	workDir := w.workDir
	if fileNotExist(workDir) {
		return
	}
	err := os.RemoveAll(workDir)
	if err != nil {
		util.Warnf("Failed to remove workDir %s for BuildConfig %s due to: %v\n", workDir, name, err)
	} else {
		util.Infof("Just deleted the folder %s for BuildConfig %s\n", workDir, name)
	}
}

func (w *BuildConfigCollector) Process() int {
	bc := &w.buildConfig
	gs := w.watcher.GitSource(bc)
	if gs == nil {
		return 0
	}
	name := bc.Name
	workDir := w.workDir
	gitDir := filepath.Join(workDir, ".git")
	if stat, err := os.Stat(gitDir); err != nil || !stat.IsDir() {
		err := w.cloneRepo(gs)
		if err != nil {
			util.Warnf("Failed to clone repo for %s due to %v\n", name, err)
		}
	} else {
		err := w.pullRepo(gs)
		if err != nil {
			util.Warnf("Failed to pull repo for %s due to %v\n", name, err)
		}
	}

	count, err := w.processCommit()
	if err != nil {
		util.Warnf("Failed to process commit for %s due to %v\n", name, err)
		return 0
	}

	if useGithub {
		client := github.NewClient(nil)
		repo, _, err := client.Repositories.Get("fabric8io", "gitcontroller")
		if err != nil {
			util.Warnf("Failed to find repo for gitcontroller! %v\n", err)
		}
		if repo != nil {
			htmlUrl := repo.HTMLURL
			if htmlUrl != nil {
				util.Infof("Found repo %s\n", *htmlUrl)
			}
		}
	}
	return count
}

func (w *BuildConfigCollector) processCommit() (int, error) {
	repo, err := git.PlainOpen(w.workDir)
	if err != nil {
		return 0, err
	}
	iter, err := repo.Commits()
	if err != nil {
		return 0, err
	}
	count := 0
	process := true
	first := true
	oldestHashLastRun := w.lastGitHash
	defer iter.Close()
	for {
		commit, err := iter.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
		if commit == nil {
			break
		}

		hash := commit.Hash.String()
		if first {
			first = false
			if hash == w.firstGitHash {
				// are we starting off with the same first hash as last time in which case
				// lets wait until after we find the last hash before processing again
				process = false
			} else if len(w.firstGitHash) == 0 {
				w.firstGitHash = hash
			}
		}
		if process {
			util.Infof("Name %s commit %s : %s\n", w.name, commit.Hash, commit.Message)
			err = w.watcher.publisher.UpsertGitCommit(&w.buildConfig, commit)
			if err != nil {
				return count, err
			}
			count = count + 1
			w.lastGitHash = hash
			if count >= maxCommits {
				break
			}
		}
		if oldestHashLastRun == hash {
			// we've now passed all the previously processed commits
			process = true
		}
	}
	return count, nil
}

func (w *BuildConfigCollector) pullRepo(gs *buildapi.GitBuildSource) error {
	util.Infof("git pull on %s\n", w.name)
	binaryFile := resolveBinaryLocation("git")
	args := []string{"pull"}
	e := exec.Command(binaryFile, args...)
	e.Dir = w.watcher.workDir
	e.Stdout = os.Stdout
	e.Stderr = os.Stderr
	err := e.Run()
	if err != nil {
		util.Errorf("Unable to start git pull %v\n", err)
		return err
	}
	return nil
}

func (w *BuildConfigCollector) cloneRepo(gs *buildapi.GitBuildSource) error {
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
	namespaceDir := filepath.Join(w.watcher.workDir, bc.Namespace)
	workDir := w.workDir
	err := os.MkdirAll(namespaceDir, 0700)
	if err != nil {
		return fmt.Errorf("Unable to create namespace work directory %s due to: %v\n", namespaceDir, err)
	}
	util.Infof("Cloning repo %s ref %s for BuildConfig %s to %s\n", uri, ref, name, workDir)
	if useGoGit {
		options := git.CloneOptions{
			URL:      uri,
			Progress: os.Stdout,
		}
		if len(ref) > 0 {
			options.ReferenceName = plumbing.ReferenceName(ref)
		}
		r, err := git.PlainClone(workDir, false, &options)
		if err != nil {
			return err
		}
		gitRef, err := r.Head()
		if err != nil {
			return fmt.Errorf("Failed to find HEAD: %v", err)
		}
		hash := gitRef.Hash()
		if len(hash) > 0 {
			commit, err := r.Commit(hash)
			if err != nil {
				return fmt.Errorf("Failed to find Commit for %v due to: %v", hash, err)
			}
			fmt.Printf("found latest commit %s\n", commit)
		}
		return err
	}
	binaryFile := resolveBinaryLocation("git")
	args := []string{"clone", uri, name}
	e := exec.Command(binaryFile, args...)
	e.Dir = namespaceDir
	e.Stdout = os.Stdout
	e.Stderr = os.Stderr
	err = e.Run()
	if err != nil {
		util.Errorf("Unable to start git clone %v\n", err)
		return err
	}
	return nil
}

// lets find the executable on the PATH or in the fabric8 directory
func resolveBinaryLocation(executable string) string {
	path, err := exec.LookPath(executable)
	if err != nil || fileNotExist(path) {
		return executable
	}
	return path
}

func findExecutable(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	if m := d.Mode(); !m.IsDir() {
		return nil
	}
	return os.ErrPermission
}

func fileNotExist(path string) bool {
	return findExecutable(path) != nil
}
