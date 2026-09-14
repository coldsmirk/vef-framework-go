package api

// HTTP header keys used by the framework.
const (
	HeaderXAppID      = "X-App-ID"
	HeaderXTimestamp  = "X-Timestamp"
	HeaderXNonce      = "X-Nonce"
	HeaderXSignature  = "X-Signature"
	HeaderXAPIKey     = "X-API-Key"
	HeaderXMetaPrefix = "X-Meta-"
	// HeaderXBodyEncoding names the transport encoding applied to a request or
	// response body. The body-encoding middleware owns it on the /api surface.
	HeaderXBodyEncoding = "X-Body-Encoding"
)
