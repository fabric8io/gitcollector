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
	//flag "github.com/spf13/pflag"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

func init() {
	RootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "operate",
	Short: "Runs the gitcollector operator",
	Long:  `This command will startup the operator for the git collector`,
	RunE:  operateCommand,
}

func operateCommand(cmd *cobra.Command, args []string) error {
	fmt.Println("gitcollector operator is starting")

	f := cmdutil.NewFactory(nil)
	f.BindFlags(cmd.PersistentFlags())

	c, cfg := client.NewClient(f)
	ns, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	oc, _ := client.NewOpenShiftClient(cfg)

	bw := watcher.New(c, oc, ns)

	stopc := make(chan struct{})
	errc := make(chan error)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		if err := bw.Run(); err != nil {
			errc <- err
		}
		wg.Done()
	}()

	term := make(chan os.Signal)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	select {
	case <-term:
		fmt.Fprintln(os.Stderr)
		util.Info("Received SIGTERM, exiting gracefully.../n")
		close(stopc)
		wg.Wait()
	case err := <-errc:
		util.Warnf("Unexpected error received: %v/n", err)
		close(stopc)
		wg.Wait()
		return err
	}
	return nil
}
