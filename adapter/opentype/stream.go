package opentype

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
	"github.com/npillmayer/opentype/otshape/othebrew"
)

// Stream provides bounded output and synchronous production. It starts no
// goroutines: the consumer must drain Glyphs concurrently with Shape.
type Stream struct {
	mu              sync.Mutex
	running, closed bool
	err             error
	next            uint64
	out             chan contracts.ShapedGlyph
}

var _ contracts.StreamingShaper = (*Stream)(nil)

// NewStream creates a bounded synchronous shaper with positive channel capacity.
// It starts no goroutines; arrange concurrent draining before calling Shape.
func NewStream(capacity int) (*Stream, error) {
	if capacity <= 0 {
		return nil, otshape.ErrChannelCapacity
	}
	return &Stream{out: make(chan contracts.ShapedGlyph, capacity)}, nil
}
// Glyphs returns the output channel. Drain it concurrently with Shape.
// Finish or a terminal shaping failure closes the channel.
func (s *Stream) Glyphs() <-chan contracts.ShapedGlyph { return s.out }
// Err returns the retained terminal shaping failure, or nil, under the stream lock.
func (s *Stream) Err() error                           { s.mu.Lock(); defer s.mu.Unlock(); return s.err }
// SetInitialCluster sets the scalar cluster base for the next Shape call.
// It rejects busy or closed streams. Successful shaping advances the base by the
// number of input runes, not UTF-8 bytes.
func (s *Stream) SetInitialCluster(n uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return otshape.ErrStreamBusy
	}
	if s.closed {
		return otshape.ErrStreamClosed
	}
	s.next = uint64(n)
	return nil
}
// Finish closes output when idle and returns any retained shaping error.
// It is idempotent once closed; a busy stream returns ErrStreamBusy without closing.
func (s *Stream) Finish() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return otshape.ErrStreamBusy
	}
	if !s.closed {
		s.closed = true
		close(s.out)
	}
	return s.err
}
// Shape synchronously produces glyphs while the caller drains Glyphs concurrently.
// It accepts only a nonnil OpenType Face and holds its shared lock during shaping.
// Feature ranges and output clusters use scalar coordinates; successful calls
// advance the cluster base and leave output open for another run or Finish.
// Busy/closed calls are rejected. After starting a run, any validation, shaping or
// cancellation error is terminal: it is retained by Err and closes output. Context
// is checked during reading/output; synchronous backend work retains its limits.
func (s *Stream) Shape(ctx context.Context, p contracts.ShapeParams, src contracts.RuneReader, o contracts.ShapeBufferOptions) (err error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return otshape.ErrStreamBusy
	}
	if s.closed {
		s.mu.Unlock()
		return otshape.ErrStreamClosed
	}
	s.running = true
	base := s.next
	s.mu.Unlock()
	reader := &countedReader{ctx: ctx, src: src, base: base}
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.running = false
		if err != nil {
			s.err = err
			s.closed = true
			close(s.out)
		} else {
			s.next = base + reader.count
		}
	}()
	if ctx == nil {
		return otshape.ErrNilContext
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if src == nil {
		return otshape.ErrNilRuneSource
	}
	face, ok := p.Font.(*Face)
	if !ok || face == nil {
		return fmt.Errorf("OpenType shaper received foreign or nil face %T", p.Font)
	}
	if o.Flush != contracts.ShapeFlushRun && o.Flush != contracts.ShapeFlushCluster {
		return fmt.Errorf("unsupported flush mode %d", o.Flush)
	}
	params := otshape.Params{Font: face.font, Direction: p.Direction, Script: p.Script, Language: p.Language}
	for _, f := range p.Features {
		params.Features = append(params.Features, otshape.FeatureRange{Feature: ot.T(string(f.Tag[:])), On: f.Enabled, Arg: f.Value, Start: f.Start, End: f.End})
	}
	opts := otshape.BufferOptions{FlushBoundary: otshape.FlushBoundary(o.Flush), HighWatermark: o.HighWatermark, LowWatermark: o.LowWatermark, MaxBuffer: o.MaxBuffer}
	face.mu.Lock()
	defer face.mu.Unlock()
	err = otshape.NewShaper(othebrew.New(), otcore.New()).Shape(params, reader, glyphSink{ctx, s.out, base}, opts)
	if err == nil {
		err = ctx.Err()
	}
	return err
}

type countedReader struct {
	ctx         context.Context
	src         contracts.RuneReader
	base, count uint64
}

// ReadRune checks cancellation and tracks scalar positions without cluster overflow.
func (r *countedReader) ReadRune() (rune, int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, 0, err
	}
	c, n, err := r.src.ReadRune()
	if err == nil {
		if r.base+r.count > math.MaxUint32 {
			return 0, 0, otshape.ErrClusterOverflow
		}
		r.count++
	}
	return c, n, err
}

type glyphSink struct {
	ctx  context.Context
	out  chan contracts.ShapedGlyph
	base uint64
}

// WriteGlyph rebases scalar clusters and emits bounded output with cancellation.
// Missing glyphs and cluster overflow terminate shaping.
func (s glyphSink) WriteGlyph(g otshape.GlyphRecord) error {
	if s.base+uint64(g.Cluster) > math.MaxUint32 {
		return otshape.ErrClusterOverflow
	}
	if g.GID == 0 {
		return fmt.Errorf("missing glyph (.notdef) at original scalar %d", s.base+uint64(g.Cluster))
	}
	v := contracts.ShapedGlyph{GlyphID: uint32(g.GID), Cluster: uint32(s.base + uint64(g.Cluster)), XAdvance: g.Pos.XAdvance, YAdvance: g.Pos.YAdvance, XOffset: g.Pos.XOffset, YOffset: g.Pos.YOffset}
	select {
	case s.out <- v:
		return nil
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}
