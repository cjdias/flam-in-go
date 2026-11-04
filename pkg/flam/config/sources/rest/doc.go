// Package rest is the configuration source driver of the REST endpoints:
// its sources request a document with an
// [github.com/cjdias/flam-in-go/pkg/flam.IConfigRestClient], decode it with
// an [github.com/cjdias/flam-in-go/pkg/flam.IConfigParser] and take the bag
// found at a path of the decoded document, as a
// [github.com/cjdias/flam-in-go/pkg/flam.IConfigReloadableSource].
//
// A source is configured with:
//   - [ConfigFieldURI]: the absolute URI of the endpoint, required;
//   - [ConfigFieldMethod]: the HTTP method of the request, GET by default;
//   - [ConfigFieldHeaders]: a bag of the header values of the request, such
//     as {"Authorization": "Bearer token"}, a "Host" header setting the host
//     the request is sent with;
//   - [ConfigFieldPath]: the dot separated path, in the decoded document,
//     of the bag holding the configuration, such as "data.config", the
//     whole document when empty, by default;
//   - [ConfigFieldTimestampPath]: the dot separated path, in the decoded
//     document, of the timestamp of the configuration, such as
//     "meta.updated_at", none when empty, by default;
//   - [ConfigFieldTimeout]: the time.Duration, or a string such as "10s",
//     bounding each request, from its sending to the end of the reading of
//     the response, 30 seconds by default, zero leaving the client as the
//     only bound;
//   - [ConfigFieldMaxSize]: the maximum size, in bytes, of a response
//     body, 10 MiB by default, a larger body failing with [ErrTooLarge];
//   - [ConfigFieldParser]: the id of the parser of the response body,
//     taken from the
//     [github.com/cjdias/flam-in-go/pkg/flam.IConfigParserFactory];
//   - [ConfigFieldPriority]: the priority of the source;
//   - [ConfigFieldReload]: whether the source is reloaded, true by default.
//     When false, the source is created as a
//     [github.com/cjdias/flam-in-go/pkg/flam.IConfigSource] only, not
//     implementing
//     [github.com/cjdias/flam-in-go/pkg/flam.IConfigReloadableSource], so
//     that the reloader of the configuration skips it and its values never
//     change.
//
// The parser, priority, reload, timeout and maximum size fields, when
// missing, take the value stored at [DefaultParserConfigPath],
// [DefaultPriorityConfigPath], [DefaultReloadConfigPath],
// [DefaultTimeoutConfigPath] or [DefaultMaxSizeConfigPath], set by the
// bundle to no parser, 0, true, 30 seconds and 10 MiB. Without a parser,
// the parser is named after the media type of the Content-Type of each
// response: json for
// application/json or a type ending with +json, yaml for application/yaml,
// application/x-yaml, text/yaml or a type ending with +yaml.
//
// A response whose status is not 2xx fails the request. The configuration
// is requested when the source is created, and again by Reload, when
// activated.
//
// # Timestamp
//
// With a timestamp path, every response must hold, at that path, a string
// or a number marking the version of its configuration, such as
// "2026-10-29T10:00:00Z" or 1793268000, or the request fails with
// [ErrTimestamp]. A Reload whose response holds the timestamp of the
// values last loaded keeps them, reporting no change, without looking at
// the configuration path: an endpoint changing its configuration without
// changing its timestamp is not seen until the timestamp changes. The
// timestamps are compared for equality, not order, so that a configuration
// rolled back to an older timestamp is loaded too, and a timestamp decoded
// as another type, such as 1 and 1.0, is a change. The endpoint is still
// requested and its response parsed, as the timestamp is part of it.
//
// # Client
//
// Each source sends its requests with the client created by the
// [github.com/cjdias/flam-in-go/pkg/flam.IConfigRestClientCreator] of the
// container, once, when the source is created. The bundle provides one
// creating an [net/http.Client] with a timeout of 30 seconds; an
// application replaces it with a decorator, to configure the client, or a
// test, to fake the endpoint. The timeout of the source bounds the requests
// of any client, the one of a replacement included, unless zero, which
// leaves the client as the only bound: a client with no timeout of its own
// then waits forever for a stalled endpoint, blocking Reload.
package rest
