# GOPOD
#### _A commandline based podcatcher, written in Go (golang)_
---

* [Technology & Compatibility](#technology--compatibility)
* [Features](#features)
* [Commandline options](#commandline-options)
  * [Basic](#basic-options-gopod---help)
  * [Update](#update-options-gopod---help-update)
  * [Check Downloads](#check-downloads-options-gopod---help-checkdownloads)
  * [Preview](#preview-options-gopod---help-preview)
  * [Delete](#delete-gopod---help-delete)
* [Config File](#config-file)
  * [General configuration options](#general-configuration-options)
  * [Feed entry options](#feed-entry-options)
  * [Filename parsing options](#filename-parsing-options)
* [Gopod directory structure](#gopod-directory-structure)
  * [Log files](#log-files)
  * [Database file](#database-file)
  * [Download directories](#download-directories)
  * [Feed xml files](#feed-xml-files)
* [Why gopod?](#why)
* [Future](#future)

---
## Technology & Compatibility

Gopod is single threaded (for now); it runs thru its functions, then exits.  No scheduled tasks; if that is needed, setup some operating system specific thing for that.

On the backend, its using [GORM](https://gorm.io) for data storage with [sqlite](https://www.sqlite.org/index.html); using an [sqlite browser](https://sqlitebrowser.org) to peek at the data is definitely possible.

Gopod is written with pure Go; while it does use [sqlite](https://www.sqlite.org/index.html) as its backend, its using a [pure go sqlite driver](https://github.com/glebarez/go-sqlite) for this, to avoid CGO compatibility. Because it is pure go, just compiling on the target platform _should_ work; however I've only run it on Windows based platforms. If any issues arise from running on other platforms, create an issue. Or feel free to fix the issue yourself. :smile:

---
## Features
* todo: if I ever get to putting together a feature list.. meh

---
## Commandline Options

The following are options available to gopod on the commandline; each are available via `--help` switch.

### Basic options (`gopod --help`)

Basic options. Lists all the commands available, and the generic options that can apply to all commands
<details>

```
SYNOPSIS:
    gopod.exe --config|-c <config.toml> [--debug|--dbg] [--feed|-f <shortname>]
              [--help|-h|-?] [--proxy|-p|-- proxy <string>] <command> [<args>]

COMMANDS:
    checkdownloads    check integrity of database and files
    deletefeed        delete feed and all items from database (performs a soft delete)
    preview           preview feed file naming, based solely on feed xml.  Does not require feed existing
    update            update feeds

REQUIRED PARAMETERS:
    --config|-c <config.toml>       TOML config to use

OPTIONS:
    --debug|--dbg                   Debug (default: false)

    --feed|-f <shortname>           feed to compile on (use shortname) (default: "")

    --help|-h|-?                    (default: false)

    --proxy|-p|-- proxy <string>    use proxy url (default: "")

Use 'gopod.exe help <command>' for extra details.
```

</details>

### Update options (`gopod --help update`)

Update downloads any new (or not downloaded) podcast files. Ability to use previously downloaded podcast feed (most recent) is available.  Simulate will not download any files, or make changes to the database (useful for troubleshooting)
<details>

```
NAME:
    gopod.exe update - update feeds

SYNOPSIS:
    gopod.exe update --config|-c <config.toml> [--debug|--dbg]
                     [--feed|-f <shortname>] [--force] [--help|-h|-?]
                     [--proxy|-p|-- proxy <string>] [--set-downloaded]
                     [--simulate|--sim]
                     [--use-recent|--use-recent-xml|--userecent] [<args>]

REQUIRED PARAMETERS:
    --config|-c <config.toml>                    TOML config to use

OPTIONS:
    --debug|--dbg                                Debug (default: false)

    --feed|-f <shortname>                        feed to compile on (use shortname) (default: "")

    --force                                      force update on xml and items (will process everything in feed (default: false)

    --help|-h|-?                                 (default: false)

    --proxy|-p|-- proxy <string>                 use proxy url (default: "")

    --set-downloaded                             set already downloaded files as downloaded in db (default: false)

    --simulate|--sim                             Simulate; will not download items or save database (default: false)

    --use-recent|--use-recent-xml|--userecent    Use the most recent feed xml file fetched rather than checking for new (default: false)
```

</details>

### Check Downloads options (`gopod --help checkDownloads`)
Check Downloads checks the integrity of the files in the database, with the ability to make changes based on included commandline options.  Integrity checks include:

* Setting files to archived (if removed from download directory)
* checking filename collisions between feed entries (if any exists)
* checking or renaming files, if the filename parsing has changed
* handling of filename collisions, if occurred
<details>

```
NAME:
    gopod.exe checkdownloads - check integrity of database and files

SYNOPSIS:
    gopod.exe checkdownloads --config|-c <config.toml> [--archive|--arc]
                             [--collision|--coll] [--debug|--dbg]
                             [--feed|-f <shortname>] [--help|-h|-?]
                             [--proxy|-p|-- proxy <string>] [--rename]
                             [--savecollision|--savecoll] [<args>]

REQUIRED PARAMETERS:
    --config|-c <config.toml>       TOML config to use

OPTIONS:
    --archive|--arc                 set missing downloads to archived (default: false)

    --collision|--coll              Collision handling; will prompt for which item to keep (default: false)

    --debug|--dbg                   Debug (default: false)

    --feed|-f <shortname>           feed to compile on (use shortname) (default: "")

    --help|-h|-?                    (default: false)

    --proxy|-p|-- proxy <string>    use proxy url (default: "")

    --rename                        perform rename on files dependant on Filename parse (useful when parse value changes (default: false)

    --savecollision|--savecoll      Save collision differences to <workingdir>\.collisions\ (default: false)
```

</details>

### Preview options (`gopod --help preview`)
Preview is basically used to check file naming conventions; most useful when adding a new feed to the configuration.  Will output the potential podcast episode filenames for a feed to the console, as well as save the filename lists to `<shortname>.preview.xml`.
<details>

```
NAME:
    gopod.exe preview - preview feed file naming, based solely on feed xml.  Does not require feed existing

SYNOPSIS:
    gopod.exe preview --config|-c <config.toml> [--debug|--dbg]
                      [--feed|-f <shortname>] [--help|-h|-?]
                      [--proxy|-p|-- proxy <string>]
                      [--use-recent|--use-recent-xml|--userecent] [<args>]

REQUIRED PARAMETERS:
    --config|-c <config.toml>                    TOML config to use

OPTIONS:
    --debug|--dbg                                Debug (default: false)

    --feed|-f <shortname>                        feed to compile on (use shortname) (default: "")

    --help|-h|-?                                 (default: false)

    --proxy|-p|-- proxy <string>                 use proxy url (default: "")

    --use-recent|--use-recent-xml|--userecent    Use the most recent feed xml file fetched rather than checking for new (default: false)
```

</details>

### Delete (`gopod --help delete`)
Delete feed will delete the feed and all relevant podcast episodes from the database.  The delete is a soft delete; only marking the entries as "deleted"; no entries are actually removed.  Once a delete is performed, the user should remove the feed entry from the config file (gopod will warn if still used when deleted).
<details>

```
NAME:
    gopod.exe deletefeed - delete feed and all items from database (performs a soft delete)

SYNOPSIS:
    gopod.exe deletefeed --config|-c <config.toml> [--debug|--dbg]
                         [--feed|-f <shortname>] [--help|-h|-?]
                         [--proxy|-p|-- proxy <string>] [<args>]

REQUIRED PARAMETERS:
    --config|-c <config.toml>       TOML config to use

OPTIONS:
    --debug|--dbg                   Debug (default: false)

    --feed|-f <shortname>           feed to compile on (use shortname) (default: "")

    --help|-h|-?                    (default: false)

    --proxy|-p|-- proxy <string>    use proxy url (default: "")
```

</details>

---
## Config File
Below is a short description of the configuration options available in the config file; see the [sample config file](https://github.com/werelord/gopod/blob/main/config.example.toml) for an example.

### General configuration options
The options available in the general configurations for gopod: 
```
[config]
logfilesretained = 3   # number of logfiles (error and all, individually) to keep; 0 to disable, -1 to retain all
dupcheckmax = 8        # number of duplicate items found before skipping remaining episodes in pod rss feed.. -1 to check every entry
xmlfilesretained = 3   # number of xml files saved on disk for future reference; 0 to disable, -1 to save all
```

### Feed entry options
An example entry for a feed is shown as such:
```
[[feed]]
name = "This Week in Tech"
shortname = "twit"
url = "https://feeds.twit.tv/twit.xml"
filenameParse = "#shortname.ep#count#.mp3"
```
Options for a feed are as follows:
* `name` - Name of the feed; whatever you want; user friendly name
* `shortname` - file friendly name of the feed; used in directory and/or filenames.. Should match your file system's naming rules
* `url` - the RSS/XML/Atom url of the feed
* `filenameParse` - the rules for naming each episode of the feed.  See [Filename parsing options](#filename-parsing-options) below for details.
* `regex` - regular expression string, if filename parsing has title regex included. See [Filename parsing options](#filename-parsing-options) below for details.
* `cleanReplacement` - If episode title is used in file names, this character is used to replace characters that are not valid in file names. If omitted from configuration, will use `_` (underscore) as the replacement character.
* `episodePad` - in number-based file naming options, how many leading `0`s (zeros) will be used for that number. If omitted, default is 3 (i.e. if episode 42, filename would use "042")

### Filename parsing options

[File naming can be tricky](https://martinfowler.com/bliki/TwoHardThings.html); in my own experience, there's great variance into how various podcasts name their filenames in their urls. Some are very uniform (:heart: [cppcast](https://cppcast.com), except for those first 6 eps), and rarely deviate from that; others vary slightly; others (for tracking purposes) use GUIDs for each filename; and some even name their files for each and every episode exactly the same varied only by their url path (I'm looking at you simplecast; "content-disposition" is not a good option IMO). Even if their naming is uniform, the possibility of variation (via moving to a new content provider, or tracking system) is enough where for most every feed, I do not trust their filename naming conventions; and since I'm a packrat having a quick glance knowing whats in a directory is useful.

If filenameParse is not specified, the filename will be the same as defined in the episode's url. Note that this may cause filename collisions (which gopod will attempt to handle).

If any of the following tags are in `filenameParse`, but cannot fill the replacement for whatever reason (missing in xml, regex mismatch,etc.), gopod will use the date (prefer xml `pubDate`, or current date/time) as the replacement, formatted as `YYYYMMDD_HHMMSS`.

With this in mind, the following options for filenameParse config option are available; any of these can be combined together in any way desired:

* `#shortname#` - will insert the feed's shortname into the url
* `#urlfilename#` - will use the filename specified by the episode url (note will include the extension if used)
* `#date#` - will insert the episode publish date (defined in the feed xml) in the filename (or use current date/time if it doesn't exit); formatting for the date will be `YYYYMMDD_HHMMSS`
* `#count#` - this will use gopod's internal count of episodes. This might differ from the podcast feed's episode numbering, but is guaranteed to exists (where the podcast's feed is not)
* `#episode#` - the episode string defined in the episode's `<itunes:episode>` tag.  Note even if most episodes have this entry, there is a chance that any specific episode might still not have it defined; will use date if missing.
* `#season#` - the season string defined in the `<itunes:season>` tag; few feeds I've seen have this, but its available if desired
* `#title#` - the title of the episode.  Couple caveats exist where using title:
  * Title may have characters that are [not allowed as filename characters ](https://en.wikipedia.org/wiki/Filename#Reserved_characters_and_words); those characters will be replaced by underscores (by default) or the character defined in `cleanReplacement`
  * Note that due to possible filename length restrictions the filename may be truncated.
* `#titleregex:?#` - use in combination with the `regex` feed option above; constructing a regex in that option (with submatches) allows gopod to use those submatches in the filename generated.  Replace the `?` in the string with the number of the submatch desired (1 indexed, as the golang regexp package has index 0 as the full match of the string). See [`config.example.toml`](https://github.com/werelord/gopod/blob/main/config.example.toml) for an example.

By default, gopod will check for filename collisions in downloading new episodes; this may happen more often when a podcast feed updates an existing episode while keeping the same guid hash.  In those cases, gopod will append an character (A thru L) at the end of the filename to avoid the collision.  

---
## Gopod directory structure

Gopod's configuration file details the general non-commandline options available, as well as the details for the feeds that gopod will check and update.  See the [example config](https://github.com/openai/whisper) for an example.

Gopod will use the directory the config file is located for saving all downloaded files, log files, and the database. 

### Log files

Log files will be saved individually in the `<configDir>\.logs\` directory, while also creating a symlink in the config directory for both all log messages (`gopod.all.latest.log`) as well as error/warning logs specifically (`gopod.error.latest.log`); these symlinks will be updated to the latest log file on each run.  Each log file in the `.logs\` directory will be named with the timestamp of when gopod was run.  The number of log files to retain can be configured in the config file (see below).

### Database file

The Sqlite database file is located in the directory `<configDir>\.db\gopod.db`; This is your typical Sqlite db file; and can be explored (or modified) with whatever sqlite tool you want.

### Download directories

Gopod will save all the files downloaded from each feed in the config file directory, specifically in `<configDir>\<shortname>` directory for each feed.  Each file will be named as specified in the config file below (denoted by the `filenameparse` parameter, see below).  File timestamps (modified, specifically) are set to the pubdate for the episode in the feed.

### Feed xml files

If specified, gopod will save the feed's xml files in the shortname directory for that feed; specifically in `<configDir>\<shortname>\.xml\`.  Each xml file will be timestamped with the last time it was retrieved.  The number of files to retain can be specified in the config file; see below.

---
## Why?
*Its not like there aren't a plethora of podcatchers on \<insert mobile platform\>. Why gopod?*

My focus was two-fold on this project:
* First, learn Go and get a feel for how the language and syntax worked. Given that I rewrote the underlying database engine three times (first flat json files, second nosql database, thirdly sqlite); changed the model a few times (requiring migration), definitely got a handle of some golang quirks.  I haven't gotten into the go threading models (goroutines) yet; future, maybe
* Secondly, I'm a digital packrat; I've kept a personal library of my listened podcasts for no other reason than that packrat mentality[^1]. Rather than my previous haphazard commandline scripts for downloading podcast episodes; here we are.

Gopod is mostly a toy project; I don't have any overarching designs beyond my own personal use.  If others find usefulness in gopod, more power to you.

---
## Future

Stuff that should be done, sometime.. 

* More unit tests (better function design for better unit tests)
* When adding new feed, only download subset of files
* Embed feed/episode info & images in mp3 tags
* clean up log messages
* use goroutines, maybe
* archive flag on feed, reducing update check (if a podcast has gone on hiatus, but possibly not dead)
* domain/path change but file remains the same ((hash collision, guid == guid, url != url) shouldn't redownload; check enclosure length)

---

[^1]: who knows, maybe someday some [new technology](https://github.com/openai/whisper) would come around and allow me to run my archive of hockey podcasts to pull out notable [Jeff Marek](https://twitter.com/jeffmarek) quotes... or something...
