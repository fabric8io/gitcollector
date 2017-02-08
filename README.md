# gitcollector 

gitcollector is used to collect metadata about projects and to provide integration into the Work Item Tracker and Elasticsearch

## Work Item Tracker collected

The following URIs are populated on the Work Item Tracker REST API

* `/api/userspace/kubernetes/{namespace}/buildconfigs/{buildConfigName}` PUTs the BuildConfig resource for the namespace and buildConfigName as JSON
* `/api/userspace/git/commits/{namespace}/buildConfig{buildConfigName}/{hash}` PUTs git commits for a BuildConfig in a Namespace as JSON


## Running locally

To build it locally assuming you've got a recent install of golang and glide then get the source and set things up as follows:

    git clone https://github.com/fabric8io/gitcollector.git
    cd gitcollector
    make bootstrap  
  
Then to build and run things type:
    
    make && ./build/gitcollector operate -x

It will use your current OpenShift login and namespace. Use `oc login` to switch clusters etc     