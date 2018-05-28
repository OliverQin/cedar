# Cedar: an encrypted proxy

[![Build Status](https://api.travis-ci.org/OliverQin/cedar.png?branch=master)](https://travis-ci.org/OliverQin/cedar)
[![GoDoc](https://godoc.org/github.com/OliverQin/cedar?status.png)](https://godoc.org/github.com/OliverQin/cedar)
[![Coverage](https://codecov.io/gh/OliverQin/cedar/branch/master/graph/badge.svg)](https://codecov.io/gh/OliverQin/cedar)
=======

## Why cedar?
Unlike many other encrypted tunnel proxies, Cedar does not create connections between server and client _upon request_. Instead, it maintains several persistent connections and encapsulate network traffic into them. This is helpful in any of the following situations:

* TCP connection may be interrupted/reset by your ISP, thus no longer reliable, but the application you are using provides no retry mechanism
* Your ISP has traffic policy limiting transfer rate of each connection

## Setup

TODO: reorder the folder by Golang convention

Get the package:
```bash
go get "github.com/xxx"
```

Run the commands with `-h` to read help info.
```bash
cdrserver -c sample_config.json  # Start server
cdrlocal -c sample_config.json  # Start local
```

A sample config file:
```json
{
	"local": "127.0.0.1:1080",
	"remote": "127.0.0.1:33322",
	"password": "change_me",
	"buffersize": 100,
	"numofconns": 20
}
```

## Warning
This project is experimental and still working in progress. **Use at your own risk.**

