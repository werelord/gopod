package main

//--------------------------------------------------------------------------
import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"gopod/commandline"
	"gopod/logger"
	"gopod/pod"
	"gopod/podconfig"
	"gopod/podutils"

	"github.com/DavidGamba/go-getoptions"
	log "github.com/sirupsen/logrus"
)

// --------------------------------------------------------------------------
var (
	runTimestamp time.Time
)

const (
	Version = "v0.1.5-beta"
)

// --------------------------------------------------------------------------
func init() {

	runTimestamp = time.Now()

}

// --------------------------------------------------------------------------
func main() {

	fmt.Printf("gopod %v\n", Version)

	var (
		cmdline  *commandline.CommandLine
		poddb    *pod.PodDB
		config   *podconfig.Config
		tomlList []podconfig.FeedToml

		err error
	)

	if cmdline, err = commandline.InitCommandLine(os.Args[1:]); err != nil {
		// if help called, no errors to output
		if errors.Is(err, getoptions.ErrorHelpCalled) == false {
			fmt.Println("failed to init commandline:", err)
		}
		return
	}

	if err := logger.InitLogging(filepath.Dir(cmdline.ConfigFile), "gopod", runTimestamp); err != nil {
		fmt.Println("failed to initialize logging: ", err)
		return
	}

	// logging initialized, lets output commandline struct
	log.Debugf("cmdline: %+v", cmdline)

	if config, tomlList, err = podconfig.LoadToml(cmdline.ConfigFile, runTimestamp); err != nil {
		log.Errorf("failed to read toml file: %v", err)
		return
	}

	// settings passed from commandline
	config.CommandLineOptions = cmdline.CommandLineOptions

	log.Infof("using config: %+v", config)

	// todo: official poddb migration methods
	if poddb, err = setupDB(config); err != nil {
		log.Error("Failed setting up db: ", err)
		return
	}
	pod.Init(config, poddb)

	//------------------------------------- DEBUG -------------------------------------
	if config.Debug {
		runTest(*config, tomlList, poddb)
	}
	//------------------------------------- DEBUG -------------------------------------

	if len(cmdline.Proxy) > 0 {
		setProxy(cmdline.Proxy)
	}

	var cmdFunc commandFunc
	if cmdFunc = parseCommand(cmdline.Command); cmdFunc == nil {
		log.Error("command not recognized (this should not happen)")
		return
	}

	log.Debugf("running command: '%v'", cmdline.Command)

	cmdFunc(cmdline.FeedShortname, tomlList)

	// rotate the log files
	if config.LogFilesRetained > 0 {
		logger.RotateLogFiles(config.LogFilesRetained)
	}
}

// --------------------------------------------------------------------------
func runTest(config podconfig.Config, tomlList []podconfig.FeedToml, db *pod.PodDB) {
	if config.Debug && false {

		if pod, err := genFeed("pt", tomlList); err != nil {
			log.Errorf("well shit: %v", err)
		} else {
			log.Debugf("runtest %v", pod.Shortname)
			// pod.RunTest()
		}

		os.Exit(0)
	}
}

// --------------------------------------------------------------------------
func setupDB(cfg *podconfig.Config) (*pod.PodDB, error) {
	// dbpath := filepath.Join(cfg.WorkspaceDir, ".db", "gopod_test.db")
	dbpath := filepath.Join(cfg.WorkspaceDir, ".db", "gopod.db")

	// todo: don't do backup until a write actually happens to the db; then do it just before..
	if cfg.BackupDb && (cfg.Simulate == false) {
		// todo: do a rotate??
		var backupFile = filepath.Join(cfg.WorkspaceDir, ".db", fmt.Sprintf("gopod.bak.%s.db", cfg.TimestampStr))
		if _, err := podutils.CopyFile(dbpath, backupFile); err != nil {
			return nil, err
		} else {
			// make sure the modified/created time stay the same
			if dbStat, err := os.Stat(dbpath); err != nil {
				log.Warnf("Error getting db stats for backup: %v", err)
			} else {
				podutils.Chtimes(backupFile, dbStat.ModTime(), dbStat.ModTime())
			}
		}
	}

	if db, err := pod.NewDB(dbpath); err != nil {
		return nil, err
	} else {
		return db, nil
	}
}

// --------------------------------------------------------------------------
func setProxy(urlstr string) {
	if len(urlstr) > 0 {
		// setting default transport proxy.. don't care about the error on parse,
		if proxyUrl, err := url.ParseRequestURI(urlstr); err != nil {
			log.Error("Failed to parse proxy url: ", err)
		} else if proxyUrl != nil {
			http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
		}
	}
}

// --------------------------------------------------------------------------
func parseCommand(cmd commandline.CommandType) commandFunc {
	switch cmd {
	case commandline.Update:
		return runUpdate
	case commandline.CheckDownloaded:
		return runCheckDownloads
	case commandline.Delete:
		return runDelete
	case commandline.Preview:
		return runPreview
	case commandline.Export:
		return runExport
	default:
		return nil
	}
}

// --------------------------------------------------------------------------
func genFeedList(shortname string, tomlList []podconfig.FeedToml) ([]*pod.Feed, error) {
	if shortname != "" {
		if feed, err := genFeed(shortname, tomlList); err != nil {
			return nil, err
		} else {
			return []*pod.Feed{feed}, nil
		}
	} else {
		var feedList = make([]*pod.Feed, 0, len(tomlList))
		for _, toml := range tomlList {

			if feed, err := pod.NewFeed(toml); err != nil {
				return nil, fmt.Errorf("failed to create new feed: %v", err)
			} else {
				feedList = append(feedList, feed)
			}
		}
		return feedList, nil
	}
}

// --------------------------------------------------------------------------
func genFeed(shortname string, tomlList []podconfig.FeedToml) (*pod.Feed, error) {
	// extra check
	if shortname == "" {
		return nil, fmt.Errorf("shortname cannot be blank")
	}
	for _, toml := range tomlList {
		// shortname is optional; if it is, it's based on name
		if toml.Shortname == shortname || toml.Name == shortname {
			// convert
			return pod.NewFeed(toml)
		}
	}
	return nil, fmt.Errorf("cannot find feed config with shortname '%v'", shortname)
}

// command functions
// --------------------------------------------------------------------------
type commandFunc func(string, []podconfig.FeedToml)

func runUpdate(shortname string, tomlList []podconfig.FeedToml) {

	var res pod.DownloadResults

	if feedList, err := genFeedList(shortname, tomlList); err != nil {
		log.Error(err)
		return
	} else if len(feedList) == 0 {
		log.Error("no feeds found to update (check config or passed-in shortname)")
		return
	} else {
		res = pod.UpdateFeeds(feedList...)
	}

	// output success
	for feedShortname, fileList := range res.Results {
		fmt.Printf("%v:\n", feedShortname)
		for _, file := range fileList {
			fmt.Printf("\t%v\n", file)
		}
	}

	// output totals
	fmt.Printf("Downloaded %v files, %v\n", res.TotalDownloaded, podutils.FormatBytes(res.TotalDownloadedBytes))

	// output errors
	if len(res.Errors) > 0 {
		log.Errorf("Errors in updating feeds:\n")
		for _, err := range res.Errors {
			log.Errorf("\t%v\n", err)
		}
	}
}

// --------------------------------------------------------------------------
func runCheckDownloads(shortname string, tomlList []podconfig.FeedToml) {

	if feedList, err := genFeedList(shortname, tomlList); err != nil {
		log.Error(err)
		return
	} else if len(feedList) == 0 {
		log.Error("no feeds found to check downloads (check config or passed-in shortname)")
		return
	} else {
		for _, f := range feedList {
			if err := f.CheckDownloads(); err != nil {
				log.Errorf("Error in checking downloads for feed '%v': %v", f.Shortname, err)
			}
		}
	}
}

// --------------------------------------------------------------------------
func runDelete(shortname string, tomlList []podconfig.FeedToml) {

	if shortname == "" {
		log.Error("cannot only run delete on one feed at a time")
		return
	} else if f, err := genFeed(shortname, tomlList); err != nil {
		log.Error(err)
		return
	} else {
		// todo: logging of what's deleted
		log.WithField("feed", f.Shortname).Infof("running delete")
		if err := f.RunDelete(); err != nil {
			log.WithFields(log.Fields{
				"feed":  f.Shortname,
				"error": err,
			}).Error("failed running delete")
		}
	}
}

// --------------------------------------------------------------------------
func runPreview(shortname string, tomlList []podconfig.FeedToml) {
	if shortname == "" {
		log.Error("cannot only run preview on one feed at a time")
		return
	} else if f, err := genFeed(shortname, tomlList); err != nil {
		log.Error(err)
		return
	} else {
		log.WithField("feed", f.Shortname).Info("running preview")
		if err := f.Preview(); err != nil {
			log.WithFields(log.Fields{
				"feed":  f.Shortname,
				"error": err,
			}).Error("failed running preview")
		}
	}
}

// --------------------------------------------------------------------------
func runExport(shortname string, tomlList []podconfig.FeedToml) {

	if feedList, err := genFeedList(shortname, tomlList); err != nil {
		log.Error(err)
		return
	} else if len(feedList) == 0 {
		log.Error("no feeds found to check downloads (check config or passed-in shortname)")
	} else if err := pod.Export(feedList); err != nil {
		log.Errorf("Error in exporting feeds: %v", err)
	}
}
