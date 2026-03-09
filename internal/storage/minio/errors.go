package minio

import "errors"

// ErrUnsupportedHTTPMethod indicates unsupported HTTP method for presigned URL.
var ErrUnsupportedHTTPMethod = errors.New("unsupported HTTP method")
