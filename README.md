# Go Vanity URLs

Go Vanity URLs is a simple Go server that allows you
to set custom import paths for your Go packages.
It also can run on Google App Engine.

## Prerequesites

Install and run the binary:

```
$ go get -u github.com/GoogleCloudPlatform/govanityurls
$ # update vanity.yaml
$ govanityurls
$ # open http://localhost:8080
```


### Google App Engine

Install [gcloud](https://cloud.google.com/sdk/downloads) and install Go App Engine component:

```shell
gcloud components install app-engine-go
```

Get the application:
```
git clone https://github.com/GoogleCloudPlatform/govanityurls
cd govanityurls
```

Edit `vanity.yaml` to add any git repos:

```yaml
paths:
  /portmidi:
    repo: https://github.com/aporeto-inc/something
    private: true # skip that if you want it to show publicly (does not mean public will have access to the repo)
```

## Deploy

```shell
gcloud app deploy --project aporetodev
```
