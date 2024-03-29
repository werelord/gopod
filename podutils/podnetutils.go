package podutils

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	log "gopod/multilogger"
)

type OnResponseFunc func(resp *http.Response)
type GenRequestFunc func(string) (*http.Request, error)

type Downloader struct {
	Delay        time.Duration
	Client       *http.Client
	lastResponse time.Time
	genReqFunc   GenRequestFunc
}

// Download performs unbuffered fetches; for use for relatively short expected responses
func Download(url string) ([]byte, error) {
	var dl = Downloader{Client: &http.Client{}}
	return dl.Download(url)
}

// Download performs unbuffered fetches; for use for relatively short expected responses
func (dl *Downloader) Download(url string) (body []byte, err error) {

	// we're going to store the entire thing into buffer regardless
	// make sure result is at least empty string
	var result = new(bytes.Buffer)

	_, err = dl.dload(url, result, nil)

	return result.Bytes(), err
}

// DownloadBuffered performs buffered fetches of url
func DownloadBuffered(url string, writer io.Writer, onResp OnResponseFunc) (int64, error) {
	var dl = Downloader{Client: &http.Client{}}
	return dl.dload(url, writer, onResp)
}

func (dl *Downloader) DownloadBuffered(url string, writer io.Writer, onResp OnResponseFunc) (int64, error) {
	return dl.dload(url, writer, onResp)
}

// --------------------------------------------------------------------------
func (dl *Downloader) dload(url string, outWriter io.Writer, onResp OnResponseFunc) (bytes int64, err error) {

	// pause the download if we need to.. if the distance greater than delay, sleep should return immediately
	var (
		dist = time.Since(dl.lastResponse)
		req  *http.Request
		resp *http.Response
	)

	if (dl.Delay > 0) && (dl.Delay > dist) {
		log.Infof("(down) delay %v not passed; sleeping for %v", dl.Delay, dl.Delay-dist)
		time.Sleep(dl.Delay - dist)
	}

	if dl.Client == nil {
		// just use a default client
		dl.Client = &http.Client{}
	}
	if dl.genReqFunc == nil {
		dl.genReqFunc = createRequest
	}

	if req, err = dl.genReqFunc(url); err != nil {
		log.Errorf("failed creating request: %v", err)
		return
	}

	resp, err = dl.Client.Do(req)
	if err != nil {
		log.Errorf("Failed to download: %v", err)
		return
	}
	defer func() {
		resp.Body.Close()
		// save last response time
		dl.lastResponse = time.Now()
	}()

	log.Debugf("response status: %v", resp.Status)
	// assuming http handler automatically follows redirects; we're only checking for 200-ish status codes
	if (resp.StatusCode < http.StatusOK) || (resp.StatusCode >= http.StatusMultipleChoices) {
		err = fmt.Errorf("failed to download; response status code: %v", resp.Status)
		return
	}

	// if any handling needs outside this func
	if onResp != nil {
		onResp(resp)
	}

	// make sure its buffered, at least here..
	var bufWriter = bufio.NewWriter(outWriter)

	bytes, err = io.Copy(bufWriter, resp.Body)
	bufWriter.Flush()

	if err != nil {
		log.Errorf("error downloading: %v", err)
		return
	}

	return
}

// --------------------------------------------------------------------------
func createRequest(url string) (req *http.Request, err error) {

	if req, err = http.NewRequest("GET", url, nil); err == nil {
		// req.Header.Add("Accept", `text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8`)
		req.Header.Add("Accept", "*/*")
		req.Header.Add("Referer", "")

		// user agent taken from Vivaldi 5.3.2679.68 (Stable channel) (32-bit)
		req.Header.Add("User-Agent", `Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/102.0.5005.149 Safari/537.36`)
	}
	return
}
