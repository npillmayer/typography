package typography

import "io"

// FontSource supplies retained bytes without exposing a mutable buffer or a path.
// Implementations return an independent reader each time; callers close it.
type FontSource interface{ Open() io.ReadCloser }
