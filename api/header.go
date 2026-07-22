package api

// HTTP header keys used by the framework.
const (
	HeaderXAppID      = "X-App-ID"
	HeaderXTimestamp  = "X-Timestamp"
	HeaderXNonce      = "X-Nonce"
	HeaderXSignature  = "X-Signature"
	HeaderXAPIKey     = "X-API-Key"
	HeaderXMetaPrefix = "X-Meta-"
	// HeaderXBodyEncoding names an opt-in transport encoding the client applied
	// to the request body so it survives middleboxes that false-positive on
	// code-shaped payloads. The body-encoding middleware decodes it back to the
	// raw JSON before parsing; storage never sees the encoded form.
	HeaderXBodyEncoding = "X-Body-Encoding"
)
