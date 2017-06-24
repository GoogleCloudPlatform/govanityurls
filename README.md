# Go Vanity URLs

Go Vanity URLs is a simple App Engine Go app that allows you
to set custom import paths for your Go packages.

## Quickstart

Install [gcloud](https://cloud.google.com/sdk/downloads) and install Go App Engine component:

```
$ gcloud components install app-engine-go
```

Setup a [custom domain](https://cloud.google.com/appengine/docs/standard/python/using-custom-domains-and-ssl) for your app.

Get the application:
```
go get -u -d github.com/GoogleCloudPlatform/govanityurls
cd $(go env GOPATH)/github.com/GoogleCloudPlatform/govanityurls
```

Edit `app.yaml` with your custom domain and git repo information.

```
env_variables:
  DOMAIN: go.grpcutil.org
  REPO: https://github.com/rakyll/grpcutil
```

Deploy the app:

```
$ gcloud app deploy
```
