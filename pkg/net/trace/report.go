package trace

// reporter trace reporter.
type reporter interface {
	WriteSpan(sp *Span) error
	Close() error
}
