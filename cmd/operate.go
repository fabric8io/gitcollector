//  Copyright 2016 Red Hat, Inc.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/fabric8io/gitcollector/pkg/client"
	"github.com/fabric8io/gitcollector/pkg/util"
	"github.com/fabric8io/gitcollector/pkg/watcher"
	"github.com/spf13/cobra"

	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

func init() {
	RootCmd.AddCommand(newOperateCommand())
}

func newOperateCommand() *cobra.Command {
	p := &watcher.WatchFlags{}
	cmd := &cobra.Command{
		Use:   "operate",
		Short: "Runs the gitcollector operator",
		Long:  `This command will startup the operator for the git collector`,
		Run: func(cmd *cobra.Command, args []string) {
			err := operateCommand(cmd, args, p)
			handleError(err)
		},
	}
	f := cmd.Flags()
	f.StringVarP(&p.WorkDir, "workdir", "w", "./workdir", "the directory to store work files like git clones")
	f.StringVarP(&p.Namespace, "namespace", "n", "", "the namespace to watch")
	f.BoolVarP(&p.ExternalGitUrl, "externalGitUri", "x", false, "should we use the external git URLs when cloning")
	return cmd
}

func operateCommand(cmd *cobra.Command, args []string, p *watcher.WatchFlags) error {
	fmt.Println("gitcollector operator is starting")

	initSchema()

	f := cmdutil.NewFactory(nil)
	f.BindFlags(cmd.PersistentFlags())

	c, cfg := client.NewClient(f)
	oc, _ := client.NewOpenShiftClient(cfg)

	if len(p.Namespace) == 0 {
		n, _, err := f.DefaultNamespace()
		if err != nil {
			return err
		}
		p.Namespace = n
	}
	bw := watcher.New(c, oc, p)

	stopc := make(chan struct{})
	errc := make(chan error)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		if err := bw.Run(stopc); err != nil {
			errc <- err
		}
		wg.Done()
	}()

	term := make(chan os.Signal)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	select {
	case <-term:
		fmt.Fprintln(os.Stderr)
		util.Info("Received SIGTERM, exiting gracefully...\n")
		close(stopc)
		wg.Wait()
	case err := <-errc:
		util.Warnf("Unexpected error received: %v\n", err)
		close(stopc)
		wg.Wait()
		return err
	}
	return nil
}
