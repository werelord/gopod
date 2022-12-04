# GOPOD
#### _A commandline based podcast downloader, written in Go (golang)_
---

* [Features](#features)
* [Why gopod?](#why?)
* [Technology & Compatibility](#technology--compatibility)
* [Configuration](#configuration)
* [Commandline options](#commandline-options)
* [Future](#future)

---
## Why?
My focus was two-fold on this project:
* First, learn Go and get a feel for how the language and syntax worked. Given that I rewrote the underlying database engine three times (first flat json files, second nosql database, thirdly sqlite); changed the model a few times (requiring migration), definitely got a handle of some golang quirks.  I haven't gotten into the go threading models (goroutines) yet; future, maybe
* Secondly, I'm a digital packrat; I've kept a personal library of my listened podcasts for no other reason than that packrat mentality[^1]. Rather than my previous haphasard commandline scripts for downloading podcast episodes; here we are.

Gopod is mostly a toy project; I don't have any overarching designs beyond my own personal use.  If others find usefulness in gopod, more power to you.

## Technology & Compatibility

Gopod is single threaded (for now); it runs thru its functions, then exits.  No scheduled tasks; if that is needed, setup some operating system specific thing for that.

On the backend, its using [GORM](https://gorm.io) for data storage with [sqlite](https://www.sqlite.org/index.html); using an [sqlite browser](https://sqlitebrowser.org) to peek at the data is definitely possible.

Gopod is written with pure Go; while it does use [sqlite](https://www.sqlite.org/index.html) as its backend, its using a [pure go sqlite driver](https://github.com/glebarez/go-sqlite) for this, to avoid CGO compatibility. Because it is pure go, just compiling on the target platform _should_ work; however I've only run it on Windows based platforms. If any issues arise from running on other platforms, create an issue. Or feel free to fix the issue yourself. :smile:

## Features
* todo

## Commandline Options


## Configuration


## Future

## Dependencies

## License

[^1]: who knows, maybe someday some [new technology](https://github.com/openai/whisper) would come around and allow me to run my archive of hockey podcasts to pull out notable [Jeff Marek](https://twitter.com/jeffmarek) quotes... or something...