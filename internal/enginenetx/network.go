package enginenetx

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/ooni/probe-cli/v3/internal/bytecounter"
	"github.com/ooni/probe-cli/v3/internal/model"
	"github.com/ooni/probe-cli/v3/internal/netxlite"
	"github.com/ooni/probe-cli/v3/internal/runtimex"
	"golang.org/x/net/publicsuffix"
)

// Network is the network abstraction used by the OONI engine.
type Network struct {
	reso  model.Resolver
	stats *HTTPSDialerStatsManager
	txp   model.HTTPTransport
}

// HTTPTransport returns the [model.HTTPTransport] that the engine should use.
func (n *Network) HTTPTransport() model.HTTPTransport {
	return n.txp
}

// NewHTTPClient is a convenience function for building a [model.HTTPClient] using
// the underlying [model.HTTPTransport] and the correct cookies configuration.
func (n *Network) NewHTTPClient() *http.Client {
	// Note: cookiejar.New cannot fail, so we're using runtimex.Try1 here
	return &http.Client{
		Transport: n.txp,
		Jar: runtimex.Try1(cookiejar.New(&cookiejar.Options{
			PublicSuffixList: publicsuffix.List,
		})),
	}
}

// Close ensures that we close idle connections and persist statistics.
func (n *Network) Close() error {
	// TODO(bassosimone): do we want to introduce "once" semantics in this method?

	// make sure we close the transport's idle connections
	n.txp.CloseIdleConnections()

	// same as above but for the resolver's connections
	n.reso.CloseIdleConnections()

	// make sure we sync stats to disk
	return n.stats.Close()
}

// NewNetwork creates a new [*Network] for the engine. This network MUST NOT be
// used for measuring because it implements engine-specific policies.
//
// You MUST call the Close method when done using the network. This method ensures
// that (i) we close idle connections and (ii) persist statistics.
//
// Arguments:
//
// - counter is the [*bytecounter.Counter] to use.
//
// - kvStore is a [model.KeyValueStore] for persisting stats;
//
// - logger is the [model.Logger] to use;
//
// - proxyURL is the OPTIONAL proxy URL;
//
// - resolver is the [model.Resolver] to use.
//
// The presence of the proxyURL will cause this function to possibly build a
// network with different behavior with respect to circumvention. If there is
// an upstream proxy we're going to trust it is doing circumvention for us.
func NewNetwork(
	counter *bytecounter.Counter,
	kvStore model.KeyValueStore,
	logger model.Logger,
	proxyURL *url.URL,
	resolver model.Resolver,
) *Network {
	// Create a dialer ONLY used for dialing unencrypted TCP connections. The common use
	// case of this Network is to dial encrypted connections. For this reason, here it is
	// reasonably fine to use the legacy sequential dialer implemented in netxlite.
	dialer := netxlite.NewDialerWithResolver(logger, resolver)

	// Create manager for keeping track of statistics
	stats := NewHTTPSDialerStatsManager(kvStore, logger)

	// Create a TLS dialer ONLY used for dialing TLS connections. This dialer will use
	// happy-eyeballs and possibly custom policies for dialing TLS connections.
	httpsDialer := NewHTTPSDialer(
		logger,
		&netxlite.Netx{Underlying: nil}, // nil means using netxlite's singleton
		newHTTPSDialerPolicy(kvStore, logger, resolver),
		stats,
	)

	// Here we're creating a "new style" HTTPS transport, which has less
	// restrictions compared to the "old style" one.
	//
	// Note that:
	//
	// - we're enabling compression, which is desiredable since this transport
	// is not made for measuring and compression is good(TM);
	//
	// - if proxyURL is nil, the proxy option is equivalent to disabling
	// the proxy, otherwise it means that we're using the ooni/oohttp library
	// to dial for proxies, which has some restrictions.
	//
	// In particular, the returned transport uses dialer for dialing with
	// cleartext proxies (e.g., socks5 and http) and httpsDialer for dialing
	// with encrypted proxies (e.g., https). After this has happened,
	// the code currently falls back to using the standard library's tls
	// client code for establishing TLS connections over the proxy. The main
	// implication here is that we're not using our custom mozilla CA for
	// validating TLS certificates, rather we're using the system's cert store.
	//
	// Fixing this issue is TODO(https://github.com/ooni/probe/issues/2536).
	txp := netxlite.NewHTTPTransportWithOptions(
		logger, dialer, httpsDialer,
		netxlite.HTTPTransportOptionDisableCompression(false),
		netxlite.HTTPTransportOptionProxyURL(proxyURL),
	)

	// Make sure we count the bytes sent and received as part of the session
	txp = bytecounter.WrapHTTPTransport(txp, counter)

	netx := &Network{
		reso:  resolver,
		stats: stats,
		txp:   txp,
	}
	return netx
}

// newHTTPSDialerPolicy contains the logic to select the [HTTPSDialerPolicy] to use.
func newHTTPSDialerPolicy(kvStore model.KeyValueStore, logger model.Logger, resolver model.Resolver) HTTPSDialerPolicy {
	// the fallback policy we're using is the "null" policy
	fallback := &HTTPSDialerNullPolicy{logger, resolver}

	// make sure we honor a user-provided policy
	policy, err := NewHTTPSDialerStaticPolicy(kvStore, fallback)
	if err != nil {
		return fallback
	}

	return policy
}