package nwebconnectivity

import (
	"context"
	"net/url"
	"strings"

	"github.com/ooni/probe-cli/v3/internal/engine/geolocate"
	"github.com/ooni/probe-cli/v3/internal/engine/httpx"
	"github.com/ooni/probe-cli/v3/internal/engine/model"
	"github.com/ooni/probe-cli/v3/internal/errorsx"
)

// ControlRequest is the request that we send to the control
type ControlRequest struct {
	HTTPRequest        string              `json:"http_request"`
	HTTPRequestHeaders map[string][]string `json:"http_request_headers"`
	TCPConnect         []string            `json:"tcp_connect"`
	QUICHandshake      []string            `json:"quic_handshake"`
}

// ControlTCPConnectResult is the result of the TCP connect
// attempt performed by the control vantage point.
type ControlTCPConnectResult struct {
	Status  bool    `json:"status"`
	Failure *string `json:"failure"`
}

// ControlHTTPRequestResult is the result of the HTTP request
// performed by the control vantage point.
type ControlHTTPRequestResult struct {
	BodyLength int64             `json:"body_length"`
	Failure    *string           `json:"failure"`
	Title      string            `json:"title"`
	Headers    map[string]string `json:"headers"`
	StatusCode int64             `json:"status_code"`
}

// ControlQUICHandshakeResult is the result of the QUIC handshake
// attempt performed by the control vantage point.
type ControlQUICHandshakeResult struct {
	Status  bool    `json:"status"`
	Failure *string `json:"failure"`
}

// ControlHTTP3RequestResult is the result of the HTTP/3 request
// performed by the control vantage point.
type ControlHTTP3RequestResult struct {
	Failure    *string `json:"failure"`
	StatusCode int64   `json:"status_code"`
}

// ControlDNSResult is the result of the DNS lookup
// performed by the control vantage point.
type ControlDNSResult struct {
	Failure *string  `json:"failure"`
	Addrs   []string `json:"addrs"`
	ASNs    []int64  `json:"-"` // not visible from the JSON
}

// ControlResponse is the response from the control service.
type ControlResponse struct {
	DNS           ControlDNSResult                      `json:"dns"`
	HTTPRequest   ControlHTTPRequestResult              `json:"http_request"`
	HTTP3Request  ControlHTTPRequestResult              `json:"http3_request"`
	QUICHandshake map[string]ControlQUICHandshakeResult `json:"quic_handshake"`
	TCPConnect    map[string]ControlTCPConnectResult    `json:"tcp_connect"`
}

// Control performs the control request and returns the response.
func Control(
	ctx context.Context, sess model.ExperimentSession,
	thAddr string, creq ControlRequest) (out ControlResponse, err error) {
	clnt := httpx.Client{
		BaseURL:    thAddr,
		HTTPClient: sess.DefaultHTTPClient(),
		Logger:     sess.Logger(),
	}
	// make sure error is wrapped
	err = errorsx.SafeErrWrapperBuilder{
		Error:     clnt.PostJSON(ctx, "/", creq, &out),
		Operation: errorsx.TopLevelOperation,
	}.MaybeBuild()
	(&out.DNS).FillASNs(sess)
	return
}

// FillASNs fills the ASNs array of ControlDNSResult. For each Addr inside
// of the ControlDNSResult structure, we obtain the corresponding ASN.
//
// This is very useful to know what ASNs were the IP addresses returned by
// the control according to the probe's ASN database.
func (dns *ControlDNSResult) FillASNs(sess model.ExperimentSession) {
	dns.ASNs = []int64{}
	for _, ip := range dns.Addrs {
		// TODO(bassosimone): this would be more efficient if we'd open just
		// once the database and then reuse it for every address.
		asn, _, _ := geolocate.LookupASN(ip)
		dns.ASNs = append(dns.ASNs, int64(asn))
	}
}

func findTestHelper(e model.ExperimentSession) (testhelper *model.Service) {
	testhelpers, _ := e.GetTestHelpersByName("web-connectivity")
	for _, th := range testhelpers {
		if th.Type == "https" {
			testhelper = &th
			break
		}
	}
	return testhelper
}

// discoverH3Server inspects the Alt-Svc Header of the HTTP (over TCP) response of the control measurement
// to check whether the server announces to support h3
func (m *Measurer) discoverH3Server(resp *ControlHTTPRequestResult, URL *url.URL) (h3 bool) {
	if URL.Scheme != "https" {
		return false
	}
	alt_svc := resp.Headers["Alt-Svc"]
	entries := strings.Split(alt_svc, ";")
	for _, e := range entries {
		if strings.Contains(e, "h3") {
			return true
		}
	}
	return false
}
