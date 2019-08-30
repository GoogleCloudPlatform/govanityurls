# Go Vanity URLs

Go Vanity URLs is a simple App Engine Go app that allows you
to set custom import paths for your Go packages.

## Prerequesites

Install [gcloud](https://cloud.google.com/sdk/downloads) and install Go App Engine component:

```shell
gcloud components install app-engine-go
```

## Adding a repo

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
