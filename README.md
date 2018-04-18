# GCache2

[![GoDoc](https://godoc.org/github.com/aaronwinter/gcache2?status.png)](https://godoc.org/github.com/aaronwinter/gcache2)
[![Go Report Card](https://goreportcard.com/badge/github.com/aaronwinter/gcache2)](https://goreportcard.com/report/github.com/aaronwinter/gcache2) [![Waffle.io - Columns and their card count](https://badge.waffle.io/aaronwinter/gcache2.svg?columns=all)](https://waffle.io/aaronwinter/gcache2)

**Work-in-Progress**: This repository will be the home of gcache2, a caching library for Golang. Ideally the only one you will ever need. To reach that ambitious goal, I am taking some time to  scope and re-architecture gcache such that it becomes easier to add features, maintain existing ones, and get total testing coverage of the execution paths that matter. If you are interested in joining this effort, take a look at the issues! Stay tuned, friends. (:

## Overview

The only embedded caching library you will ever need, in Golang.

## Features

A variety of concurrency-safe caches are available:

- LFU (Least-Frequently-Used)

- LRU (Least-Recently-Used)

- ARC (Adaptive-Replacement policy)

- TinyLFU ("clever" *admission* policy)

- RR (RandomReplacement)

Other features:

* Expirable entries

* Optional support for event handlers (on eviction, purge, set, loader).

* Cache snapshots


## Install

```
$ go get github.com/aaronwinter/gcache2
```

# Authors

**Erwan Ounn** (main contributor/maintainer of [gcache2](https://github.com/aaronwinter/gcache2))

* <http://github.com/aaronwinter>
* <erwan.ounn.84@gmail.com>

**Jun Kimura** (main contributor of [gcache](https://github.com/bluele/gcache))

* <http://github.com/bluele>
* <junkxdev@gmail.com>
