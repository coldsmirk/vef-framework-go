package api

// Version vocabulary for declaring a resource's API version via WithVersion.
// VersionV1 is the framework default; V2..V9 are the reserved version ladder
// downstream applications use to declare higher API versions
// (e.g. api.WithVersion(api.VersionV3)). validateVersion also accepts any
// literal matching ^v\d+$.
const (
	VersionV1 = "v1"
	VersionV2 = "v2"
	VersionV3 = "v3"
	VersionV4 = "v4"
	VersionV5 = "v5"
	VersionV6 = "v6"
	VersionV7 = "v7"
	VersionV8 = "v8"
	VersionV9 = "v9"
)
