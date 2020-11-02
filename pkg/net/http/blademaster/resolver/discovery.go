package resolver

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/djienet/kratos/pkg/conf/env"
	"github.com/djienet/kratos/pkg/log"
	"github.com/djienet/kratos/pkg/naming"
	"github.com/djienet/kratos/pkg/naming/discovery"
	nmd "github.com/djienet/kratos/pkg/net/metadata"
)

// ResolverTransport wraps a RoundTripper.
type ResolverTransport struct {

	// The actual RoundTripper to use for the request. A nil
	// RoundTripper defaults to http.DefaultTransport.
	http.RoundTripper
}

type funcOpt struct {
	f func(*naming.BuildOptions)
}

func (f *funcOpt) Apply(opt *naming.BuildOptions) {
	f.f(opt)
}

func pick(info *naming.InstancesInfo) (host string) {
	targets := []*naming.Instance{}
	for zone, inst := range info.Instances {
		// 默认只选择当前机房的服务
		if zone == env.Zone {
			targets = inst
		}
	}

	if len(targets) == 0 {
		return
	}

	// 随机负载均衡
	rand.Seed(time.Now().UnixNano())
	inst := targets[rand.Intn(len(targets))]

	for _, a := range inst.Addrs {
		u, err := url.Parse(a)
		if err == nil && u.Scheme == "http" {
			return u.Host
		}
	}
	return
}

func filter(color string) naming.BuildOpt {
	return &funcOpt{f: func(opt *naming.BuildOptions) {
		opt.Filter = func(inss map[string][]*naming.Instance) map[string][]*naming.Instance {
			newInss := make(map[string][]*naming.Instance)
			for zone := range inss {
				var instances []*naming.Instance
				for _, ins := range inss[zone] {
					// 如果有路由染色，优先选择染色实例
					if color != "" {
						if ins.Metadata[naming.MetaColor] != color {
							continue
						}
					}

					var addr string
					for _, a := range ins.Addrs {
						u, err := url.Parse(a)
						if err == nil && u.Scheme == "http" {
							addr = u.Host
						}
					}
					if addr == "" {
						fmt.Fprintf(os.Stderr, "resolver: app(%s,%s) no valid http address(%v) found!", ins.AppID, ins.Hostname, ins.Addrs)
						log.Warn("resolver: invalid http address(%s,%s,%v) found!", ins.AppID, ins.Hostname, ins.Addrs)
						continue
					}
					instances = append(instances, ins)
				}
				newInss[zone] = instances
			}
			return newInss
		}
	}}
}

// NewResolverTransport NewResolverTransport
func NewResolverTransport(rt http.RoundTripper) *ResolverTransport {
	return &ResolverTransport{RoundTripper: rt}
}

func (t *ResolverTransport) resolve(ctx context.Context, appID string, builder naming.Builder) (info *naming.InstancesInfo, err error) {
	color := nmd.String(ctx, nmd.Color)
	if color == "" && env.Color != "" {
		color = env.Color
	}

	resolver := builder.Build(appID, filter(color))

	ev := resolver.Watch()
	_, ok := <-ev
	if !ok {
		err = errors.New("discovery watch failed")
		return
	}

	info, ok = resolver.Fetch(context.Background())
	if !ok {
		err = errors.New("discovery poll nodes fail")
		return
	}

	return
}

func (t *ResolverTransport) roundTrip(rt http.RoundTripper, req *http.Request, builder naming.Builder) (*http.Response, error) {
	// url format: discovery://appid/xxxx
	newReq := new(http.Request)
	*newReq = *req

	ctx := req.Context()
	info, err := t.resolve(ctx, req.URL.Hostname(), builder)
	if err != nil {
		return nil, err
	}

	host := pick(info)
	if host == "" {
		return nil, errors.New("discovery pick error: unknown host")
	}

	newReq.Host = host
	newReq.URL.Scheme = "http"
	newReq.URL.Host = host

	resp, err := rt.RoundTrip(newReq)
	if err != nil && resp != nil {
		resp.Request.Host = req.Host
		resp.Request.URL.Scheme = req.URL.Scheme
		resp.Request.URL.Host = req.URL.Host
	}

	return resp, err
}

// RoundTrip implements the RoundTripper interface
func (t *ResolverTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt := t.RoundTripper
	if rt == nil {
		rt = http.DefaultTransport
	}

	// discovery://appid/xxx
	if req.URL.Scheme == "discovery" {
		return t.roundTrip(rt, req, discovery.Builder())
	}

	// default to http resolve
	return rt.RoundTrip(req)
}
