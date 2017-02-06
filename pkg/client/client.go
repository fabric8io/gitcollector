/**
 * Copyright (C) 2015 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *         http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package client

import (
	"github.com/fabric8io/gitcontroller/util"
	oclient "github.com/openshift/origin/pkg/client"
	"k8s.io/kubernetes/pkg/client/restclient"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

func NewClient(f *cmdutil.Factory) (*client.Client, *restclient.Config) {
	var err error
	cfg, err := f.ClientConfig()
	if err != nil {
		util.Error("Could not initialise a client - is your server setting correct?\n\n")
		util.Fatalf("%v", err)
	}
	c, err := client.New(cfg)
	if err != nil {
		util.Fatalf("Could not initialise a client: %v", err)
	}

	return c, cfg
}

func NewOpenShiftClient(cfg *restclient.Config) (*oclient.Client, *restclient.Config) {
	ocfg := *cfg
	ocfg.APIPath = ""
	c, err := oclient.New(&ocfg)
	if err != nil {
		util.Fatalf("Could not initialise an OpenShift client: %v", err)
	}

	return c, cfg
}
