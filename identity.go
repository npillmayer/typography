package typography

import "github.com/npillmayer/khipu/contracts"

// IdentityProvider supplies backend-independent identity for retained faces.
// It performs no discovery and must not reopen a source path.
type IdentityProvider interface {
	Identify(contracts.FontFace) (Info, error)
}
