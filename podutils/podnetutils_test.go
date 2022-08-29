package podutils

import (
	"bytes"
	"fmt"
	"gopod/testutils"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"testing"
)

type serverConfig struct {
	removeHost bool
	serverPath string
	statusCode int
	response   string
	headers    map[string]string
}

func createServer(cfg serverConfig) *httptest.Server {
	var server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if cfg.headers != nil {
			for k, v := range cfg.headers {
				// possibility of non-canonical header values; set the map directly
				(w.Header())[k] = []string{v}
			}
		}
		// for some reason if setting the response status code before headers are inserted
		// they're never copied.. make sure this is the last entry after headers
		w.WriteHeader(cfg.statusCode)
		w.Write([]byte(cfg.response))
	}))
	var url, _ = url.Parse(server.URL)
	if cfg.serverPath != "" {
		url.Path = path.Join(url.Path, cfg.serverPath)
	}

	if cfg.removeHost {
		// for hacking shit
		url.Host = ""
	}
	// back to string
	server.URL = url.String()

	return server
}

func TestDownload(t *testing.T) {

	tests := []struct {
		name     string
		srvResp  serverConfig
		wantBody string
		wantErr  bool
	}{
		{
			"bad url (no host)",
			serverConfig{removeHost: true, statusCode: http.StatusOK, response: "foobar"},
			"",
			true,
		},
		{
			"3XX error", // redirects shouldn't appear unless stuff changes, then this test will fail
			serverConfig{statusCode: http.StatusMovedPermanently, response: "foobar"},
			"",
			true,
		},
		{
			"4XX error",
			serverConfig{statusCode: http.StatusForbidden, response: "foobar"},
			"",
			true,
		},
		{
			"5XX error",
			serverConfig{statusCode: http.StatusInternalServerError, response: "foobar"},
			"",
			true,
		},
		{
			"success",
			serverConfig{statusCode: http.StatusOK, response: "foobar"},
			"foobar",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server = createServer(tt.srvResp)
			defer server.Close()

			gotBody, err := Download(server.URL)
			testutils.AssertErr(t, tt.wantErr, err)
			testutils.AssertEquals(t, tt.wantBody, string(gotBody))
		})
	}
}

func TestDownloadBuffered(t *testing.T) {

	tests := []struct {
		name    string
		srvResp serverConfig
		wantBuf string
		wantErr bool
	}{
		{
			"bad url (no host)",
			serverConfig{removeHost: true, statusCode: http.StatusOK, response: "foobar"},
			"",
			true,
		},
		{
			"3XX error", // redirects shouldn't appear unless stuff changes, then this test will fail
			serverConfig{statusCode: http.StatusMovedPermanently, response: "foobar"},
			"",
			true,
		},
		{
			"4XX error",
			serverConfig{statusCode: http.StatusForbidden, response: "foobar"},
			"",
			true,
		},
		{
			"5XX error",
			serverConfig{statusCode: http.StatusInternalServerError, response: "foobar"},
			"",
			true,
		},
		{
			"success",
			serverConfig{
				statusCode: http.StatusOK,
				headers:    map[string]string{"Content-Disposition": "filename=\"foobar.mp3\""},
				response:   "foobar"},
			"foobar",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var server = createServer(tt.srvResp)
			defer server.Close()

			var buf = bytes.NewBufferString("")
			bw, gotcd, err := DownloadBuffered(server.URL, buf, nil)

			testutils.AssertErr(t, tt.wantErr, err)
			testutils.Assert(t, bw == int64(len(tt.wantBuf)), fmt.Sprintf("bytes written expected to be 0; got %v", bw))
			testutils.AssertEquals(t, tt.wantBuf, buf.String())
			testutils.AssertEquals(t, tt.srvResp.headers["Content-Disposition"], gotcd)
		})
	}
}

func Test_createRequest(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			"bad url",
			"\t",
			true,
		},
		{
			"check headers",
			"http://foo.com/bar",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := createRequest(tt.url)

			testutils.AssertErr(t, tt.wantErr, err)
			if tt.wantErr {
				testutils.Assert(t, req == nil, fmt.Sprintf("request should be nil, got %v", req))
			} else {
				var exists bool
				// make sure specific headers exist; all else outside of scope
				_, exists = req.Header["Accept"]
				testutils.Assert(t, exists, "req missing Accept header")
				_, exists = req.Header["User-Agent"]
				testutils.Assert(t, exists, "req missing User-Agent header")
			}
		})
	}
}
