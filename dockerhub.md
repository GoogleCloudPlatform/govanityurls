
## travis ci 密码配置
```
$ travis encrypt DOCKER_EMAIL=email@gmail.com --add
$ travis encrypt DOCKER_USER=username --add
$ travis encrypt DOCKER_PASS=password --add
```


reference
----
[Using Travis.ci to build Docker images](https://sebest.github.io/post/using-travis-ci-to-build-docker-images/)




## k8s deploy
由于govanityurls需要使用使用80端口，而k8s ingress使用了80端口，如果把govanityurls部署在k8s中，则需要创建Ingress。创建文件参考`k8s-deployment.yaml`。

创建完成后使用浏览器、curl、wget访问可以正常获取数据，但是使用go get却出现错误：
```
# go get -v -insecure icp.inspur.com/trident
Fetching https://icp.inspur.com/trident?go-get=1
Parsing meta tags from https://icp.inspur.com/trident?go-get=1 (status code 404)
package icp.inspur.com/trident: unrecognized import path "icp.inspur.com/trident" (parse https://icp.inspur.com/trident?go-get=1: no go-import meta tags ())
```

查看go源代码`src/cmd/go/internal/web/http.go`：
```
// GetMaybeInsecure returns the body of either the importPath's
// https resource or, if unavailable and permitted by the security mode, the http resource.
func GetMaybeInsecure(importPath string, security SecurityMode) (urlStr string, body io.ReadCloser, err error) {
	fetch := func(scheme string) (urlStr string, res *http.Response, err error) {
		u, err := url.Parse(scheme + "://" + importPath)
		if err != nil {
			return "", nil, err
		}
		u.RawQuery = "go-get=1"
		urlStr = u.String()
		if cfg.BuildV {
			log.Printf("Fetching %s", urlStr)
		}
		if security == Insecure && scheme == "https" { // fail earlier
			res, err = impatientInsecureHTTPClient.Get(urlStr)
		} else {
			res, err = httpClient.Get(urlStr)
		}
		return
	}
	closeBody := func(res *http.Response) {
		if res != nil {
			res.Body.Close()
		}
	}
	urlStr, res, err := fetch("https")
	if err != nil {
		if cfg.BuildV {
			log.Printf("https fetch failed: %v", err)
		}
		if security == Insecure {
			closeBody(res)
			urlStr, res, err = fetch("http")
		}
	}
	if err != nil {
		closeBody(res)
		return "", nil, err
	}
	// Note: accepting a non-200 OK here, so people can serve a
	// meta import in their http 404 page.
	if cfg.BuildV {
		log.Printf("Parsing meta tags from %s (status code %d)", urlStr, res.StatusCode)
	}
	return urlStr, res.Body, nil
}
```
即使go get时增加了`-insecure` 参数，它仍然首先尝试使用https访问，而Ingress中未设置ssl证书，https的请求会被转发到default backend，其中又没有请求对应的url，所以返回了404，而go get不认为这是错误，继续解析返回的结果，导致了Parse错误。

因此需要设置Ingress的证书。

## 设置Ingress证书：
### 生成证书
```
mkdir ~/govanityurls-certs
openssl req -x509 -nodes -days 3650 -newkey rsa:2048 -keyout ~/govanityurls-certs/tls.key -out ~/govanityurls-certs/tls.crt -subj "/CN=icp.inspur.com"

kubectl create secret tls govanityurls-cert --key ~/govanityurls-certs/tls.key --cert ~/govanityurls-certs/tls.crt -n govanityurls
```

## 设置centos7 dns
```
#显示当前网络连接
#nmcli connection show
NAME             UUID                                  TYPE            DEVICE          
br-c796b0985509  0bc59e88-f906-42cb-abfa-df320ad6d2ea  bridge          br-c796b0985509 
docker0          9d02670e-cebf-4185-9090-16a00217529f  bridge          docker0         
eth0             5fb06bd0-0bb0-7ffb-45f1-d6edd65f3e03  802-3-ethernet  eth0            
tunl0            36aa96b4-def5-49a8-ab2b-e7807d4b25d8  ip-tunnel       tunl0

#修改当前网络连接对应的DNS服务器，这里的网络连接可以用名称或者UUID来标识
#nmcli con mod eth0 ipv4.dns "10.100.1.58 223.5.5.5"

#将dns配置生效
#nmcli con up eth0
```


-----
参考
https://github.com/kubernetes/contrib/tree/master/ingress/controllers/nginx/examples/tls
