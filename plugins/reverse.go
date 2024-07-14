package plugins

import (
	"TheOnlyMirror/config"
	"bytes"
	"compress/gzip"
	"golang.org/x/net/http2"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func HandlerReverse(w http.ResponseWriter, r *http.Request, source config.Source) {
	targetUrl, _ := url.Parse(source.Mirror)
	proxy := httputil.NewSingleHostReverseProxy(targetUrl)
	if r.ProtoMajor == 2 {
		proxy.Transport = &http2.Transport{}
	}
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = targetUrl.Scheme
		req.URL.Host = targetUrl.Host
		req.URL.Path = source.Prefix + strings.TrimPrefix(req.URL.Path, source.Prefix)
		req.Host = targetUrl.Host
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		if len(source.Replaces) == 0 {
			return nil
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		resp.Body.Close()
		encoding := resp.Header.Get("Content-Encoding")
		var modifiedBody string
		switch encoding {
		case "gzip":
			reader, _ := gzip.NewReader(bytes.NewBuffer(bodyBytes))
			dataDecompressing, err := ioutil.ReadAll(reader)
			if err != nil {
				modifiedBody = string(bodyBytes)
			} else {
				modifiedBody = string(dataDecompressing)
			}
		default:
			modifiedBody = string(bodyBytes)
		}
		for _, replace := range source.Replaces {
			src := replace.Src
			dst := replace.Dst
			switch dst {
			case "<HOST>":
				dst = r.Host
			case "<TLS_SCHEME>":
				if config.GetTlsRedirect() == true {
					if config.GetTls() == true {
						dst = "https://"
					} else {
						dst = "http://"
					}
				} else {
					continue
				}
			}
			switch replace.Type {
			case "header":
				header := resp.Header.Get(replace.Header)
				header = strings.Replace(header, src, dst, -1)
				resp.Header.Set(replace.Header, header)
			default:
				modifiedBody = strings.Replace(modifiedBody, src, dst, -1)
			}

		}
		switch encoding {
		case "gzip":
			var b bytes.Buffer
			gw := gzip.NewWriter(&b)
			_, _ = gw.Write([]byte(modifiedBody))
			_ = gw.Close()
			modifiedBody = string(b.Bytes())
		}
		resp.Body = io.NopCloser(strings.NewReader(modifiedBody))
		resp.ContentLength = int64(len(modifiedBody))
		return nil
	}
	proxy.ServeHTTP(w, r)
}
