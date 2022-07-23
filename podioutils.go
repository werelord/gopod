package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/flytam/filenamify"
	"github.com/schollz/progressbar/v3"
)

//--------------------------------------------------------------------------
// download unbuffered
func Download(url string) (body []byte, err error) {
	log.Debug("downloading ", url)

	var (
		req  *http.Request
		resp *http.Response
	)

	client := &http.Client{}

	if req, err = createRequest(url); err != nil {
		log.Error("failed creating request: ", err)
		return
	}

	resp, err = client.Do(req)

	log.Debugf("response status: %v", resp.Status)
	log.Debugf("err: %+v", err)

	if err != nil {
		log.Error("failed to download: ", err)
		return
	}
	defer resp.Body.Close()
	// todo: check more error codes
	if resp.StatusCode != 200 {
		log.Errorf("failed to download; response status code: %v", resp.Status)
		return
	}

	if body, err = io.ReadAll(resp.Body); err != nil {
		log.Error("failed to read response body: ", err)
		return
	}
	return

}

//--------------------------------------------------------------------------
func DownloadBuffered(url, destfile string) (err error) {

	var (
		req  *http.Request
		resp *http.Response
	)

	file, err := os.CreateTemp(filepath.Dir(destfile), filepath.Base(destfile)+"_temp*")
	if err != nil {
		log.Error("Failed creating temp file: ", err)
		return
	}
	defer file.Close()

	client := &http.Client{}

	if req, err = createRequest(url); err != nil {
		log.Error("failed creating request: ", err)
		return
	}

	// todo: combine request/response stuff
	resp, err = client.Do(req)

	if err != nil {
		log.Error("Failed to download: ", err)
		return
	}
	defer resp.Body.Close()
	log.Debugf("response status: %v", resp.Status)
	// todo: check more error codes
	if resp.StatusCode != 200 {
		log.Errorf("failed to download; response status code: %v", resp.Status)
		return
	}

	bar := progressbar.NewOptions64(resp.ContentLength,
		progressbar.OptionSetDescription(path.Base(destfile)),
		progressbar.OptionFullWidth(),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() { fmt.Fprint(os.Stderr, "\n") }),
		progressbar.OptionSetTheme(progressbar.Theme{Saucer: "=", SaucerHead: ">", SaucerPadding: " ", BarStart: "[", BarEnd: "]"}))

	podWriter := bufio.NewWriter(file)
	b, err := io.Copy(io.MultiWriter(podWriter, bar), resp.Body)
	podWriter.Flush()
	if err != nil {
		log.Error("error in writing file: ", err)
		return
	} else {
		log.Debugf("file written {%v} bytes: %.2fKB", path.Base(file.Name()), float64(b)/(1<<10))
	}
	// explicit close
	file.Close()

	// move tempfile to finished file
	if err = os.Rename(file.Name(), destfile); err != nil {
		log.Debug("error moving temp file: ", err)
		return
	}
	return
}

//--------------------------------------------------------------------------
func createRequest(url string) (req *http.Request, err error) {

	if req, err = http.NewRequest("GET", url, nil); err == nil {
		req.Header.Add("Accept", `text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8`)
		// user agent taken from Vivaldi 5.3.2679.68 (Stable channel) (32-bit)
		req.Header.Add("User-Agent", `Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/102.0.5005.149 Safari/537.36`)
	}
	return
}

//--------------------------------------------------------------------------
func cleanFilename(filename string) string {
	fname, _ := filenamify.Filenamify(filename, filenamify.Options{Replacement: "-"})
	return fname
}