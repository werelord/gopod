package podutils

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"
)

type OnResponseFunc func(resp *http.Response)

// --------------------------------------------------------------------------
// download unbuffered
func Download(url string) (body []byte, err error) {
	log.Debug("downloading ", url)

	// we're going to store the entire thing into buffer regardless
	// make sure result is at least empty string
	var result = new(bytes.Buffer)

	_, err = dload(url, result, nil)

	return result.Bytes(), err
}

func DownloadBuffered(url string, writer io.Writer, onResp OnResponseFunc) (int64, error) {
	return dload(url, writer, onResp)
}

// --------------------------------------------------------------------------
func dload(url string, outWriter io.Writer, onResp OnResponseFunc) (bytes int64, err error) {

	var (
		req  *http.Request
		resp *http.Response
	)

	// setting client timeout, see https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
	client := &http.Client{
		// Timeout: 30 * time.Second,
	}

	if req, err = createRequest(url); err != nil {
		log.Error("failed creating request: ", err)
		return
	}

	resp, err = client.Do(req)
	if err != nil {
		log.Error("Failed to download: ", err)
		return
	}
	defer resp.Body.Close()
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
	podWriter := bufio.NewWriter(outWriter)

	bytes, err = io.Copy(podWriter, resp.Body)
	podWriter.Flush()

	if err != nil {
		log.Error("error downloading: ", err)
		return
	}

	return
}

// --------------------------------------------------------------------------
func createRequest(url string) (req *http.Request, err error) {

	if req, err = http.NewRequest("GET", url, nil); err == nil {
		req.Header.Add("Accept", `text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8`)
		// user agent taken from Vivaldi 5.3.2679.68 (Stable channel) (32-bit)
		req.Header.Add("User-Agent", `Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/102.0.5005.149 Safari/537.36`)
	}
	return
}
