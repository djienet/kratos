package trace

import (
	"flag"
	"fmt"
	"os"

	"github.com/pkg/errors"

	"github.com/djienet/kratos/pkg/conf/dsn"
	"github.com/djienet/kratos/pkg/conf/env"
	xtime "github.com/djienet/kratos/pkg/time"
)

var _jaegerTraceDSN = "udp://127.0.0.1:6831"

func init() {
	if v := os.Getenv("TRACE"); v != "" {
		_jaegerTraceDSN = v
	}
	flag.StringVar(&_jaegerTraceDSN, "trace", _jaegerTraceDSN, "jaeger trace report dsn, or use TRACE env.")
}

// Config config.
type Config struct {
	// Report network e.g. unixgram, tcp, udp
	Network string `dsn:"network"`
	// For TCP and UDP networks, the addr has the form "host:port".
	// For Unix networks, the address must be a file system path.
	Addr string `dsn:"address"`
	// Report timeout
	Timeout xtime.Duration `dsn:"query.timeout,200ms"`
	// DisableSample
	DisableSample bool `dsn:"query.disable_sample"`
	// ProtocolVersion
	ProtocolVersion int32 `dsn:"query.protocol_version,1"`
	// Probability probability sampling
	Probability float32 `dsn:"-"`
}

func parseDSN(rawdsn string) (*Config, error) {
	d, err := dsn.Parse(rawdsn)
	if err != nil {
		return nil, errors.Wrapf(err, "trace: invalid dsn: %s", rawdsn)
	}
	cfg := new(Config)
	if _, err = d.Bind(cfg); err != nil {
		return nil, errors.Wrapf(err, "trace: invalid dsn: %s", rawdsn)
	}
	return cfg, nil
}

// TracerFromEnvFlag new tracer from env and flag
func TracerFromEnvFlag() (Tracer, error) {
	cfg, err := parseDSN(_jaegerTraceDSN)
	if err != nil {
		return nil, err
	}
	report := newReport(cfg.Addr)
	return NewTracer(env.AppID, report, cfg.DisableSample), nil
}

// Init init trace report.
func Init(cfg *Config) {
	if cfg == nil {
		// paser config from env
		var err error
		if cfg, err = parseDSN(_jaegerTraceDSN); err != nil {
			panic(fmt.Errorf("parse trace dsn error: %s", err))
		}
	}
	report := newReport(cfg.Addr)
	SetGlobalTracer(NewTracer(env.AppID, report, cfg.DisableSample))
}
