package font

import "io"

// Source supplies retained bytes without exposing a mutable buffer or a path.
// Implementations return an independent reader each time; callers close it.
type Source interface{ Open() io.ReadCloser }
