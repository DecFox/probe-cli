package testingproxy

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/apex/log"
	"github.com/ooni/netem"
	"github.com/ooni/probe-cli/v3/internal/netxlite"
	"github.com/ooni/probe-cli/v3/internal/runtimex"
	"github.com/ooni/probe-cli/v3/internal/testingsocks5"
	"github.com/ooni/probe-cli/v3/internal/testingx"
)

// WithNetemSOCKSProxyAndURL returns a [TestCase] where:
//
// - we fetch a URL;
//
// - using the github.com/ooni.netem;
//
// - and a SOCKS5 proxy.
//
// Because this [TestCase] uses netem, it also runs in -short mode.
func WithNetemSOCKSProxyAndURL(URL string) TestCase {
	return &netemTestCaseWithSOCKS{
		TargetURL: URL,
	}
}

type netemTestCaseWithSOCKS struct {
	TargetURL string
}

var _ TestCase = &netemTestCaseWithSOCKS{}

// Name implements TestCase.
func (tc *netemTestCaseWithSOCKS) Name() string {
	return fmt.Sprintf("fetching %s using netem and a SOCKS5 proxy", tc.TargetURL)
}

// Run implements TestCase.
func (tc *netemTestCaseWithSOCKS) Run(t *testing.T) {
	topology := runtimex.Try1(netem.NewStarTopology(log.Log))
	defer topology.Close()

	const (
		wwwIPAddr    = "93.184.216.34"
		proxyIPAddr  = "10.0.0.1"
		clientIPAddr = "10.0.0.2"
	)

	// create:
	//
	// - a www stack modeling www.example.com
	//
	// - a proxy stack
	//
	// - a client stack
	//
	// Note that www.example.com's IP address is also the resolver used by everyone
	wwwStack := runtimex.Try1(topology.AddHost(wwwIPAddr, wwwIPAddr, &netem.LinkConfig{}))
	proxyStack := runtimex.Try1(topology.AddHost(proxyIPAddr, wwwIPAddr, &netem.LinkConfig{}))
	clientStack := runtimex.Try1(topology.AddHost(clientIPAddr, wwwIPAddr, &netem.LinkConfig{}))

	// configure the wwwStack as the DNS resolver with proper configuration
	dnsConfig := netem.NewDNSConfig()
	dnsConfig.AddRecord("www.example.com.", "", wwwIPAddr)
	dnsServer := runtimex.Try1(netem.NewDNSServer(log.Log, wwwStack, wwwIPAddr, dnsConfig))
	defer dnsServer.Close()

	// configure the wwwStack to respond to HTTP requests on port 80
	wwwServer80 := testingx.MustNewHTTPServerEx(
		&net.TCPAddr{IP: net.ParseIP(wwwIPAddr), Port: 80},
		wwwStack,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Bonsoir, Elliot!\r\n"))
		}),
	)
	defer wwwServer80.Close()

	// configure the wwwStack to respond to HTTPS requests on port 443
	wwwServer443 := testingx.MustNewHTTPServerTLSEx(
		&net.TCPAddr{IP: net.ParseIP(wwwIPAddr), Port: 443},
		wwwStack,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Bonsoir, Elliot!\r\n"))
		}),
		wwwStack,
	)
	defer wwwServer443.Close()

	// configure the proxyStack to implement the SOCKS proxy on port 9050
	proxyServer := testingsocks5.MustNewServer(
		log.Log,
		&netxlite.Netx{
			Underlying: &netxlite.NetemUnderlyingNetworkAdapter{UNet: proxyStack}},
		&net.TCPAddr{IP: net.ParseIP(proxyIPAddr), Port: 9050},
	)
	defer proxyServer.Close()

	// create the netx instance for the client
	netx := &netxlite.Netx{Underlying: &netxlite.NetemUnderlyingNetworkAdapter{UNet: clientStack}}

	// create an HTTP client configured to use the given proxy
	//
	// note how we use a dialer that asserts that we're using the proxy IP address
	// rather than the host address, so we're sure that we're using the proxy
	dialer := &dialerWithAssertions{
		ExpectAddress: proxyIPAddr,
		Dialer:        netx.NewDialerWithResolver(log.Log, netx.NewStdlibResolver(log.Log)),
	}
	tlsDialer := netxlite.NewTLSDialer(dialer, netx.NewTLSHandshakerStdlib(log.Log))
	txp := netxlite.NewHTTPTransportWithOptions(log.Log, dialer, tlsDialer,
		netxlite.HTTPTransportOptionProxyURL(proxyServer.URL()),

		// TODO(https://github.com/ooni/probe/issues/2536)
		netxlite.HTTPTransportOptionTLSClientConfig(&tls.Config{
			RootCAs: runtimex.Try1(clientStack.DefaultCertPool()),
		}),
	)
	client := &http.Client{Transport: txp}
	defer client.CloseIdleConnections()

	// get the homepage and assert we're getting a succesful response
	httpCheckResponse(t, client, tc.TargetURL)
}

// Short implements TestCase.
func (tc *netemTestCaseWithSOCKS) Short() bool {
	return true
}