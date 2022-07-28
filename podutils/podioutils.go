package podutils

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/flytam/filenamify"
	"github.com/schollz/progressbar/v3"
	log "github.com/sirupsen/logrus"
)

const FILENAME_MAX_LENGTH = 200

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
	//log.Debugf("err: %+v", err)

	if err != nil {
		log.Error("failed to download: ", err)
		return
	}
	defer resp.Body.Close()
	// todo: check more error codes
	if resp.StatusCode != 200 {
		err = fmt.Errorf("download failed; response status code: %v", resp.Status)
		log.Error(err)
		return
	}

	if body, err = io.ReadAll(resp.Body); err != nil {
		log.Error("failed to read response body: ", err)
		return
	}
	return

}

//--------------------------------------------------------------------------
func DownloadBuffered(url, destfile string) (contentDisposition string, err error) {

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

	// grab content disposition, if it exists
	contentDisposition = resp.Header.Get("Content-Disposition")
	//log.Debug("Content-Disposition: ", contentDisposition)

	bar := progressbar.NewOptions64(resp.ContentLength,
		progressbar.OptionSetDescription(filepath.Base(destfile)),
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
		log.Debugf("file written {%v} bytes: %.2fKB", filepath.Base(file.Name()), float64(b)/(1<<10))
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
func CleanFilename(filename string) string {
	// only error this generates is if the replacement is a reserved character
	fname, _ := filenamify.Filenamify(filename, filenamify.Options{Replacement: "-", MaxLength: FILENAME_MAX_LENGTH})
	return fname
}

//--------------------------------------------------------------------------
func RotateFiles(path, regexPattern string, numToKeep uint) error {

	var (
		r        *regexp.Regexp
		err      error
		filelist []fs.FileInfo
	)

	if r, err = regexp.Compile(regexPattern); err != nil {
		return err
	}

	err = filepath.WalkDir(path, func(filename string, d fs.DirEntry, err error) error {
		if d.IsDir() == false {
			if r.MatchString(filename) {
				if fi, e := d.Info(); e == nil {
					filelist = append(filelist, fi)
				} else {
					log.Warn("error getting file info (not adding to list): ", e)
				}
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	// sort the resulting slice
	sort.Slice(filelist, func(i, j int) bool {
		return filelist[i].ModTime().After(filelist[j].ModTime())
	})

	// remove the X number of files beyond the limit
	for _, f := range filelist[numToKeep:] {
		log.Debug("removing file: ", f.Name())
		if err := os.Remove(filepath.Join(path, f.Name())); err != nil {
			log.Warnf("failed to remove '%v': %v", f.Name(), err)
		}
	}

	return nil
}
