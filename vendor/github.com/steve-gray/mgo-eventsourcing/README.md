[![Build Status](https://travis-ci.org/globalsign/mgo.svg?branch=master)](https://travis-ci.org/globalsign/mgo) [![GoDoc](https://godoc.org/github.com/steve-gray/mgo-eventsourcing?status.svg)](https://godoc.org/github.com/steve-gray/mgo-eventsourcing)

The MongoDB driver for Go
-------------------------

This fork has had a few improvements by ourselves as well as several PR's merged from the original mgo repo that are currently awaiting review. Changes are mostly geared towards performance improvements and bug fixes, though a few new features have been added.

Further PR's (with tests) are welcome, but please maintain backwards compatibility.

## Changes
* Fixes attempting to authenticate before every query ([details](https://github.com/go-mgo/mgo/issues/254))
* Removes bulk update / delete batch size limitations ([details](https://github.com/go-mgo/mgo/issues/288))
* Adds native support for `time.Duration` marshalling ([details](https://github.com/go-mgo/mgo/pull/373))
* Reduce memory footprint / garbage collection pressure by reusing buffers ([details](https://github.com/go-mgo/mgo/pull/229), [more](https://github.com/steve-gray/mgo-eventsourcing/pull/56))
* Support majority read concerns ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/2))
* Improved connection handling ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/5))
* Hides SASL warnings ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/7))
* Support for partial indexes ([detials](https://github.com/domodwyer/mgo/commit/5efe8eccb028238d93c222828cae4806aeae9f51))
* Fixes timezone handling ([details](https://github.com/go-mgo/mgo/pull/464)) 
* Integration tests run against MongoDB 3.2 & 3.4 releases ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/4), [more](https://github.com/steve-gray/mgo-eventsourcing/pull/24), [more](https://github.com/steve-gray/mgo-eventsourcing/pull/35))
* Improved multi-document transaction performance ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/10), [more](https://github.com/steve-gray/mgo-eventsourcing/pull/11), [more](https://github.com/steve-gray/mgo-eventsourcing/pull/16))
* Fixes cursor timeouts ([details](https://jira.mongodb.org/browse/SERVER-24899))
* Support index hints and timeouts for count queries ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/17))
* Don't panic when handling indexed `int64` fields ([detials](https://github.com/go-mgo/mgo/issues/475))
* Supports dropping all indexes on a collection ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/25))
* Annotates log entries/profiler output with optional appName on 3.4+ ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/28))
* Support for read-only [views](https://docs.mongodb.com/manual/core/views/) in 3.4+ ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/33))
* Support for [collations](https://docs.mongodb.com/manual/reference/collation/) in 3.4+ ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/37))
* Provide BSON constants for convenience/sanity ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/41))
* Consistently unmarshal time.Time values as UTC ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/42))
* Enforces best practise coding guidelines ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/44))
* GetBSON correctly handles structs with both fields and pointers ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/40))
* Improved bson.Raw unmarshalling performance ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/49))
* Minimise socket connection timeouts due to excessive locking ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/52))
* Natively support X509 client authentication ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/55))
* Gracefully recover from a temporarily unreachable server ([details](https://github.com/steve-gray/mgo-eventsourcing/pull/69))

---

### Thanks to
* @bachue
* @bozaro
* @BenLubar
* @carter2000
* @cezarsa
* @drichelson
* @eaglerayp
* @feliixx
* @fmpwizard
* @idy
* @jameinel
* @gazoon
* @mapete94
* @Reenjii
* @smoya
* @wgallagher