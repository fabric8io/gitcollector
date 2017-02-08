package publisher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/fabric8io/gitcollector/pkg/util"
	"k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildapiv1 "github.com/openshift/origin/pkg/build/api/v1"
)

type Publisher struct {
	witUrl           string
	elasticsearchUrl string
}

func New() Publisher {
	return Publisher{
		witUrl:           urlFromEnvVars("WIT"),
		elasticsearchUrl: urlFromEnvVars("ELASTICSEARCH"),
	}
}

// urlFromEnvVars uses the kubernetes FOO_SERVICE_HOST and FOO_SERVICE_PORT environment
// variables to find the services for the given name (in capitals)
func urlFromEnvVars(name string) string {
	host := os.Getenv(name + "_SERVICE_HOST")
	answer := ""
	if len(host) > 0 {
		port := os.Getenv(name + "_SERVICE_PORT")
		prefix := "http://"

		if len(port) > 0 {
			answer = prefix + host + ":" + port + "/"
		} else {
			answer = prefix + host + "/"
		}
	}
	util.Infof("Accessing %s at URL: %s\n", name, answer)
	return answer
}

func (p *Publisher) UpsertBuildConfig(bc *buildapi.BuildConfig) error {
	if bc == nil {
		return fmt.Errorf("No BuildConfig supplied!")
	}
	u1 := p.buildConfigURLForWIT(bc)
	u2 := p.buildConfigURLForES(bc)
	if len(u1) == 0 && len(u2) == 0 {
		return nil
	}

	// marshalling from a non v1 does nto generate lower case JSON
	// so lets convert to v1
	var v1BC buildapiv1.BuildConfig
	err := api.Scheme.Convert(bc, &v1BC, nil)
	if err != nil {
		return fmt.Errorf("Cannot convert from api to api/v1 of BuildConfig: %v", err)
	}

	data, err := json.Marshal(&v1BC)
	if err != nil {
		return fmt.Errorf("Failed to marshal BuildConfig to JSON: %v", err)
	}

	err = p.putJSON(u1, &data)
	if err != nil {
		return err
	}
	err = p.putJSON(u2, &data)
	return err
}

func (p *Publisher) putJSON(u string, data *[]byte) error {
	if len(u) == 0 {
		return nil
	}
	util.Infof("Putting JSON at %s\n", u)
	//util.Infof("JSON: %s\n", string(*data))
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPut, u, bytes.NewReader(*data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(*data))
	resp, err := client.Do(req)
	if err == nil {
		util.Infof("Got result %d\n", resp.StatusCode)
		return nil
	} else {
		status := -1
		if resp != nil {
			status = resp.StatusCode
		}
		return fmt.Errorf("Failed to PUT to %s with status code %d due to: %v", u, status, err)
	}
}

// buildConfigURLForWIT uses /api/userspace/kubernetes/{namespace}/buildconfigs/{buildConfig}
func (p *Publisher) buildConfigURLForWIT(bc *buildapi.BuildConfig) string {
	host := p.witUrl
	if len(host) == 0 {
		return ""
	}
	u, err := url.Parse(host)
	if err != nil {
		util.Fatalf("Cannot parse the WIT URL %s due to: %v\n", host, err)
	}
	u.Path = path.Join("/api/userspace/kubernetes", bc.Namespace, "/buildconfigs", bc.Name)
	return u.String()
}

// buildConfigURLForES uses /api/userspace/kubernetes/{namespace}/buildconfigs/{buildConfig}
func (p *Publisher) buildConfigURLForES(bc *buildapi.BuildConfig) string {
	host := p.elasticsearchUrl
	if len(host) == 0 {
		return ""
	}
	u, err := url.Parse(host)
	if err != nil {
		util.Fatalf("Cannot parse the Elasticsearch URL %s due to: %v\n", host, err)
	}
	u.Path = path.Join("/index/foo")
	return u.String()
}
