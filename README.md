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

Edit `vanity.yaml` to add any number of git repos. E.g., `customdomain.com/portmidi` will
serve the [https://github.com/rakyll/portmidi](https://github.com/rakyll/portmidi) repo.

```
/portmidi:
  repo: https://github.com/rakyll/portmidi
```

You can add as many rules as you wish.

Deploy the app:

```
$ gcloud app deploy
```

That's it! You can use `go get` to get the package from your custom domain.

```
$ go get customdomain.com/portmidi
```
