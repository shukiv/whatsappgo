package gateway

import (
	"context"
	"encoding/json"
)

// InteractiveFeatures is an optional extension for the reviewed desktop
// interactions. Implementations must reject unknown methods and validate all
// account, target and permission requirements; the UI is not a trust boundary.
type InteractiveFeatures interface {
	InteractiveFeature(context.Context, string, json.RawMessage) (any, error)
}
