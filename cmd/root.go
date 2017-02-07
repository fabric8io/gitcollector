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
	"github.com/spf13/cobra"
	//"github.com/spf13/viper"

	aapi "github.com/openshift/origin/pkg/authorization/api"
	aapiv1 "github.com/openshift/origin/pkg/authorization/api/v1"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildapiv1 "github.com/openshift/origin/pkg/build/api/v1"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployapiv1 "github.com/openshift/origin/pkg/deploy/api/v1"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	oauthapiv1 "github.com/openshift/origin/pkg/oauth/api/v1"
	projectapi "github.com/openshift/origin/pkg/project/api"
	projectapiv1 "github.com/openshift/origin/pkg/project/api/v1"
	tapi "github.com/openshift/origin/pkg/template/api"
	tapiv1 "github.com/openshift/origin/pkg/template/api/v1"
	"k8s.io/kubernetes/pkg/api"
)

const ()

var RootCmd = &cobra.Command{
	Use:   "gitcollector",
	Short: "gitcollector is a Function as a Service (Lambda) style programming model for Kubernetes",
	Long: `Funktion lets you develop complex applications using Functions and then use Flows to bind those functions to any event source (over 200 event sources and connectors supported) and run and scale your functions on top of kubernetes.

For more documentation please see: https://gitcollector.fabric8.io/`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
		}
	},
}

func init() {
	//	viper.BindPFlags(RootCmd.PersistentFlags())
	//cobra.OnInitialize(initConfig)
}

func handleError(err error) {
	if err != nil {
		fmt.Printf("Failed: %v\n", err)
	}
}

func usageError(cmd *cobra.Command, format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s\nSee '%s -h' for help and examples.", msg, cmd.CommandPath())
}

func initSchema() {
	buildapi.AddToScheme(api.Scheme)
	buildapiv1.AddToScheme(api.Scheme)
	aapi.AddToScheme(api.Scheme)
	aapiv1.AddToScheme(api.Scheme)
	tapi.AddToScheme(api.Scheme)
	tapiv1.AddToScheme(api.Scheme)
	projectapi.AddToScheme(api.Scheme)
	projectapiv1.AddToScheme(api.Scheme)
	deployapi.AddToScheme(api.Scheme)
	deployapiv1.AddToScheme(api.Scheme)
	oauthapi.AddToScheme(api.Scheme)
	oauthapiv1.AddToScheme(api.Scheme)
}
