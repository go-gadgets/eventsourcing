/*
Package keyvalue contains a base implementation of much of the common logic required for a
scale-out tablestore implementation of an event store. Each store driver need only support
four methods, which can be passed via the keyvalue.Options structure:

		CheckSequence			// Check if a particular key/seq pair exists.
		FetchEvents				// Fetch events forward from a particular sequence number
		PutEvents				// Put a set of events into the store
		Close					// Shut-down the driver

By abstracting store implementations down to this API, it's assumed it will be easier to
add more providers later. Specific providers that suit this model include DynamoDB, Azure
Tables, MongoDB, Cassandra - but the model will work for essentially any provider that has
support for a dual-part unique key (Agg ID, Sequence) and supports range scans for these.
*/
package keyvalue
