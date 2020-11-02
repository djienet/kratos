package paladin

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"sync"
	"time"

	"github.com/djienet/kratos/pkg/conf/env"
	"github.com/djienet/kratos/pkg/ecode"
	"github.com/djienet/kratos/pkg/log"
	http "github.com/djienet/kratos/pkg/net/http/blademaster"
	"github.com/djienet/kratos/pkg/net/netutil"
	xtime "github.com/djienet/kratos/pkg/time"
)

const _maxLoadRetries = 3

var (
	_ Client = &oasis{}

	_debug = false

	// 配置中心地址
	confHost string
	// 应用版本
	confVersion string
	// 配置缓存目录
	confCachePath string
)

func init() {
	flag.StringVar(&confHost, "conf_host", os.Getenv("CONF_HOST"), `config api host.`)
	flag.StringVar(&confVersion, "conf_version", os.Getenv("CONF_VERSION"), `app version.`)
	flag.StringVar(&confCachePath, "conf_cache_path", os.Getenv("CONF_CACHE_PATH"), `config file cache path.`)

	if env.DeployEnv == env.DeployEnvDev {
		_debug = true
	}

	// 未指定配置中心地址，走 discovery 服务发现
	if confHost == "" {
		confHost = "discovery://infra.config"
	}
}

type Diff struct {
	Version int64  `json:"version"`
	Name    string `json:"name"`
}

type Content struct {
	Version int64  `json:"version"`
	Content string `json:"content"`
	MD5     string `json:"md5"`
}

type oasisWatcher struct {
	keys []string
	ch   chan Event
}

func newOasisWatcher(keys []string) *oasisWatcher {
	return &oasisWatcher{keys: keys, ch: make(chan Event, 5)}
}

func (ow *oasisWatcher) HasKey(key string) bool {
	if len(ow.keys) == 0 {
		return true
	}
	for _, k := range ow.keys {
		if k == key {
			return true
		}
	}
	return false
}

func (ow *oasisWatcher) Handle(event Event) {
	select {
	case ow.ch <- event:
	default:
		log.Error("paladin: discard event:%+v", event)
	}
}

func (ow *oasisWatcher) Chan() <-chan Event {
	return ow.ch
}

func (ow *oasisWatcher) Close() {
	close(ow.ch)
}

type oasis struct {
	client   *http.Client
	values   *Map
	wmu      sync.RWMutex
	watchers map[*oasisWatcher]struct{}
	backoff  *netutil.BackoffConfig
}

func NewOasis() (Client, error) {
	a := &oasis{
		client: http.NewClient(&http.ClientConfig{
			Dial:      xtime.Duration(3 * time.Second),
			Timeout:   xtime.Duration(40 * time.Second),
			KeepAlive: xtime.Duration(40 * time.Second),
		}),
		values:   new(Map),
		watchers: make(map[*oasisWatcher]struct{}),
		backoff: &netutil.BackoffConfig{
			MaxDelay:  5 * time.Second,
			BaseDelay: 1.0 * time.Second,
			Factor:    1.6,
			Jitter:    0.2,
		},
	}

	if err := a.checkEnv(); err != nil {
		return nil, err
	}

	diffs, err := a.preload()
	if err != nil {
		return nil, err
	}

	go a.watchproc(diffs)

	return a, nil
}

func (a *oasis) checkEnv() error {
	if confVersion == "" || confCachePath == "" || env.AppID == "" {
		return fmt.Errorf("config env invalid. conf_version(%s) conf_cache_path(%s) app_id(%s)", confVersion, confCachePath, env.AppID)
	}
	return nil
}

// 配置预加载，第一次初始化时，会加载应用所有配置到本地
func (a *oasis) preload() (diffs []*Diff, err error) {
	if diffs, err = a.check(nil); err != nil {
		log.Error("paladin: check(-1) error(%v)", err)
		return
	}

	all := make(map[string]*Value, len(diffs))
	for _, diff := range diffs {
		for i := 0; i < _maxLoadRetries; i++ {
			c, err := a.get(diff)
			if err != nil {
				log.Error("paladin: get(%v) error(%v)", diff, err)
				time.Sleep(a.backoff.Backoff(i))
				continue
			}
			all[diff.Name] = &Value{val: c.Content, raw: c.Content}
			break
		}
	}
	a.values.Store(all)

	return
}

// 获取指定配置文件
func (a *oasis) get(diff *Diff) (c *Content, err error) {
	params := a.newParams()
	params.Set("name", diff.Name)
	params.Set("version", fmt.Sprintf("%d", diff.Version))

	var resp struct {
		Code    int      `json:"code"`
		Message string   `json:"message"`
		Data    *Content `json:"data"`
	}

	if _debug {
		log.Info("paladin: get params(%+v)", params)
	}
	if err = a.client.Get(context.Background(),
		confHost+"/api/v1/config/fetch", "", params, &resp); err != nil {
		return
	}

	if rc := ecode.Int(resp.Code); !ecode.Equal(rc, ecode.OK) || resp.Data == nil {
		err = fmt.Errorf("paladin: http config is nil. params(%s) ecode(%d)", params.Encode(), resp.Code)
		return
	}

	if err = ioutil.WriteFile(path.Join(confCachePath, diff.Name), []byte(resp.Data.Content), 0644); err != nil {
		return
	}

	c = resp.Data
	return
}

func (a *oasis) newParams() url.Values {
	params := url.Values{}
	params.Set("app_id", env.AppID)
	params.Set("env", env.DeployEnv)
	params.Set("zone", env.Zone)
	params.Set("build", confVersion)

	return params
}

func (a *oasis) check(diffs []*Diff) (ret []*Diff, err error) {
	var params struct {
		AppID string  `json:"app_id"`
		Env   string  `json:"env"`
		Zone  string  `json:"zone"`
		Build string  `json:"build"`
		Items []*Diff `json:"items"`
	}

	params.AppID = env.AppID
	params.Env = env.DeployEnv
	params.Zone = env.Zone
	params.Build = confVersion
	params.Items = diffs

	if diffs == nil {
		params.Items = []*Diff{}
	}
	if _debug {
		log.Info("paladin: check params(%+v)", params)
	}
	req, err := a.client.NewJSONRequest("POST", confHost+"/api/v1/config/listeners", params)
	if err != nil {
		return
	}

	var resp struct {
		Code    int     `json:"code"`
		Message string  `json:"message"`
		Data    []*Diff `json:"data"`
	}

	if err = a.client.JSON(context.Background(), req, &resp); err != nil {
		return
	}

	if rc := ecode.Int(resp.Code); !ecode.Equal(rc, ecode.OK) {
		err = rc
		return
	}
	ret = resp.Data

	return
}

func (a *oasis) watchproc(_diffs []*Diff) {
	var retry int
	for {
		diffs, err := a.check(_diffs)
		if err != nil {
			if ecode.EqualError(ecode.NotModified, err) {
				time.Sleep(time.Second)
				continue
			}
			log.Error("paladin: check(%v) error(%v)", diffs, err)
			retry++
			time.Sleep(a.backoff.Backoff(retry))
			continue
		}

		all := a.values.Load()
		news := make(map[string]*Value, len(diffs))
		for _, diff := range diffs {
			c, err := a.get(diff)
			if err != nil {
				log.Error("paladin: get(%v) error(%v)", diff, err)
				retry++
				time.Sleep(a.backoff.Backoff(retry))
				continue
			}
			if _, ok := all[diff.Name]; !ok {
				go a.fireEvent(Event{Event: EventAdd, Key: diff.Name, Value: c.Content})
			} else if c.Content != "" {
				go a.fireEvent(Event{Event: EventUpdate, Key: diff.Name, Value: c.Content})
			} else {
				go a.fireEvent(Event{Event: EventRemove, Key: diff.Name, Value: c.Content})
			}
			news[diff.Name] = &Value{val: c.Content, raw: c.Content}
		}

		for k, v := range all {
			if _, ok := news[k]; !ok {
				news[k] = v
			}
		}
		a.values.Store(news)

		for _, _diff := range _diffs {
			for _, diff := range diffs {
				if _diff.Name == diff.Name {
					_diff.Version = diff.Version
				}
			}
		}
		retry = 0 // reset
	}
}

// Get return value by key.
func (a *oasis) Get(key string) *Value {
	return a.values.Get(key)
}

// GetAll return value map.
func (a *oasis) GetAll() *Map {
	return a.values
}

func (a *oasis) fireEvent(event Event) {
	a.wmu.RLock()
	for w := range a.watchers {
		if w.HasKey(event.Key) {
			w.Handle(event)
		}
	}
	a.wmu.RUnlock()
}

// WatchEvent watch with the specified keys.
func (a *oasis) WatchEvent(ctx context.Context, keys ...string) <-chan Event {
	aw := newOasisWatcher(keys)

	a.wmu.Lock()
	a.watchers[aw] = struct{}{}
	a.wmu.Unlock()
	return aw.Chan()
}

// Close close watcher.
func (a *oasis) Close() (err error) {
	a.wmu.RLock()
	for w := range a.watchers {
		w.Close()
	}
	a.wmu.RUnlock()
	return
}
