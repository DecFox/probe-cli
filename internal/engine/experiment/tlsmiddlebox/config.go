package tlsmiddlebox

//
// Config for the tlsmiddlebox experiment
//

import (
	"net/url"
	"time"
)

// Config contains the experiment configuration.
type Config struct {
	// ResolverURL is the default DoH resolver
	ResolverURL string `ooni:"URL for DoH resolver"`

	// SNIPass is the SNI value we don't expect to be blocked
	SNIControl string `ooni:"the SNI value to cal"`

	// Delay is the delay between each iteration (in milliseconds).
	Delay int64 `ooni:"delay between consecutive iterations"`

	// Iterations is the default number of interations we trace
	MaxTTL int64 `ooni:"iterations is the number of iterations"`

	// TestHelper iis the testhelper host for iterative tracing
	TestHelper string `ooni:"the SNI value to use"`

	// ClientId is the client fingerprint to use
	ClientId int `ooni:"the ClientHello fingerprint to use"`
}

func (c Config) resolverURL() string {
	if c.ResolverURL != "" {
		return c.ResolverURL
	}
	return "https://mozilla.cloudflare-dns.com/dns-query"
}

func (c Config) snicontrol() string {
	if c.SNIControl != "" {
		return c.SNIControl
	}
	return "example.com"
}

func (c Config) delay() time.Duration {
	if c.Delay > 0 {
		return time.Duration(c.Delay) * time.Millisecond
	}
	return 100 * time.Millisecond
}

func (c Config) maxttl() int64 {
	if c.MaxTTL > 0 {
		return c.MaxTTL
	}
	return 20
}

// TODO(DecFox): We want to replace this with a generic input parser
// Issue: https://github.com/ooni/probe/issues/2239
func (c Config) testhelper(address string) (URL *url.URL, err error) {
	if c.TestHelper != "" {
		return url.Parse(c.TestHelper)
	}
	URL = &url.URL{
		Host:   address,
		Scheme: "tlshandshake",
	}
	return
}

func (c Config) clientid() int {
	if c.ClientId > 0 {
		return c.ClientId
	}
	return 0
}
