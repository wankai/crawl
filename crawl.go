// features:
// routine安全
// 定制user agent
// 支持代理
// 支持静态Cookie
// 支持重定向

package crawl

import (
	"os"
	"path/filepath"
	"errors"
	"io"
	"bufio"
	"strings"
	"net/http"
	"net/url"
	"sync"
)

func getDomain(url string) string {
	scheme := "http://"
	i := strings.Index(url, scheme);
	if i != -1 {
		url = url[len(scheme):]
	}
	i = strings.Index(url, "/")
	if i != -1 {
		url = url[0:i]
	}
	i = strings.LastIndex(url, ".")
	if i == -1 {
		return url
	}
	seg := url[0:i]
	i = strings.LastIndex(seg, ".")
	if i == -1 {
		return url
	}
	domain := url[i+1:]

	return domain
}


var defaultUserAgent = "fetch"

type ProxyList struct {
	list []string
	index int
	mutex sync.Mutex
}

var defaultProxyList = &ProxyList{}

func (pl *ProxyList) load(file string) (err error) {
	var f *os.File
	var line string

	if f, err = os.Open(file); err != nil {
		return err
	}
	r := bufio.NewReader(f)
	for {
		if line, err = r.ReadString('\n'); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		line = strings.TrimSpace(line)
		pl.list = append(pl.list, line)
	}
	return nil
}

func (pl *ProxyList) getProxy() (u *url.URL, err error) {
	pl.mutex.Lock()
	defer pl.mutex.Unlock()
	if len(pl.list) == 0 {
		return nil, nil
	}
	s := pl.list[0]
	pl.index++
	if pl.index >= len(pl.list) {
		pl.index = 0
	}
	return url.Parse(s)
}

func proxyFunc(req* http.Request)(*url.URL, error) {
	return defaultProxyList.getProxy()
}

type CookieSet struct {
	data map[string] []*http.Cookie
}

func (cs *CookieSet) SetCookies(u *url.URL, cookies []*http.Cookie) {
	domain := getDomain(u.Host)
	cs.data[domain] = cookies
}

func (cs *CookieSet) Cookies(u *url.URL) []*http.Cookie {
	domain := getDomain(u.Host)
	return cs.data[domain]
}

func (cs *CookieSet) load(dir string) error {
	var err error

	loadOne := func(dir string, info os.FileInfo, e error) error {
		var f *os.File
		var err error
		var line string

		if info.IsDir() {
			return nil
		}
		file := dir + "/" + info.Name()
		if f, err = os.Open(file); err != nil {
			return err
		}
		r := bufio.NewReader(f)
		for {
			if line, err = r.ReadString('\n'); err != nil {
				if err == io.EOF {
					break
				}
				i := strings.Index(line, "=")
				if i == -1 {
					return errors.New(file + " format error")
				}
				name := strings.TrimSpace(line[0:i])
				value := strings.TrimSpace(line[i+1:])
				if name == "" || value == "" {
					return errors.New(file + " format error")
				}
				domain := filepath.Base(dir)
				if _, ok := cs.data[domain]; !ok {
					cs.data[domain] = make([]*http.Cookie, 0)
				}
				cs.data[domain] = append(cs.data[domain], &http.Cookie{Name: name, Value: value})
			}
		}
		return nil
	}

	if err = filepath.Walk(dir, loadOne); err != nil {
		return err
	}
	return nil
}

var defaultCookieSet = &CookieSet{ data: make(map[string] []*http.Cookie) }


type Options struct {
	UserAgent string
	ProxyFile string
	CookieDir string
}

type Client struct {
	http.Client
	opt *Options
}

func NewClient(opt *Options) (c *Client, err error) {
	c = &Client{
		opt: opt,
	}

	if opt.ProxyFile != "" {
		if err = defaultProxyList.load(opt.ProxyFile); err != nil {
			return nil, err
		}
	}
	if opt.CookieDir != "" {
		if err = defaultCookieSet.load(opt.CookieDir); err != nil {
			return nil, err
		}
	}
	c.Transport = &http.Transport{Proxy: proxyFunc}
	c.Jar =  defaultCookieSet

	if opt.UserAgent == "" {
		opt.UserAgent = defaultUserAgent
	}
	return c, nil
}

func (c *Client) Get(url string, referer string) (res *http.Response, err error) {
	var req *http.Request
	opt := c.opt

	if req, err = http.NewRequest("GET", url, nil); err != nil {
		return nil, err
	}
	if len(referer) > 0 {
		req.Header.Set("Referer", referer)
	}
	if opt.UserAgent != "" {
		req.Header.Set("User-Agent", opt.UserAgent)
	}
	return c.Do(req)
}
