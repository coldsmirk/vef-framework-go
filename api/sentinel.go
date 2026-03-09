package api

// P is a sentinel type that marks a struct as API parameters.
// Embed this type in your request parameter struct to enable
// automatic decoding from Request.Params.
//
// Example:
//
//	type CreateUserParams struct {
//	    api.P
//	    Name  string `json:"name" validate:"required"`
//	    Email string `json:"email" validate:"required,email"`
//	}
type P struct{}

// M is a sentinel type that marks a struct as API metadata.
// Embed this type in your metadata struct to enable
// automatic decoding from Request.Meta.
//
// Example:
//
//	type PageMeta struct {
//	    api.M
//	    Page     int `json:"page"`
//	    Size     int `json:"size"`
//	}
type M struct{}
