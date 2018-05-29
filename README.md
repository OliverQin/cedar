# Cedar: an encrypted proxy

[![Build Status](https://api.travis-ci.org/OliverQin/cedar.png?branch=master)](https://travis-ci.org/OliverQin/cedar)
[![GoDoc](https://godoc.org/github.com/OliverQin/cedar?status.png)](https://godoc.org/github.com/OliverQin/cedar)
[![Coverage](https://codecov.io/gh/OliverQin/cedar/branch/master/graph/badge.svg)](https://codecov.io/gh/OliverQin/cedar)
=======

## Features
Unlike many other encrypted tunnel proxies, Cedar does not create connections between server and client _upon request_. 
Instead, it maintains several persistent connections and encapsulates network traffic into them. This is helpful in situations like:

* Your connection may be interrupted/reset by 3rd party (e.g. your ISP), but application layer provides no retry mechanism
* Your ISP has traffic policy limiting transfer rate by _each_ connection

## Setup

Get Cedar:
```bash
# on server side
go get "github.com/OliverQin/cedar/cmd/cdrserver"

# local
go get "github.com/OliverQin/cedar/cmd/cdrlocal"
```

Run the commands with `-h` to read help info.
```bash
# Start server, using config file
go run cdrserver.go -c sample_config.json  

# Start local SOCKS5 server
# You can use command line paramters directly, 
go run cdrlocal.go -n 20 -b 100 -p change_me -s 127.0.0.1:1080 -r 12.3.45.67:33322 -n 20
```

A sample config file:
```json
{
	"local": "127.0.0.1:1080",
	"remote": "12.3.45.67:33322",
	"password": "change_me",
	"buffersize": 100,
	"numofconns": 20
}
```

## Note

This project is experimental and still working in progress. **Use at your own risk.**

Comments and PRs are welcome!