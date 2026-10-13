package meshbvh

// Stats counts the work done by RayTest calls.  Counting is cheap enough to
// leave on: RayTest counts in locals and adds them into Stats once per call,
// without atomics, so each thread should count into its own Stats and merge
// them afterwards with Add.
type Stats struct {
	// Composite nodes visited.  Each one is a single test of the ray
	// against all of the node's children's bounds at once.
	InnerVisited int64

	// Leaf references reported to checkRef.
	LeafRefs int64
}

// NodesVisited returns the total number of nodes visited, both composite and
// leaf.
func (s *Stats) NodesVisited() int64 {
	return s.InnerVisited + s.LeafRefs
}

// Add adds o's counts into s.
func (s *Stats) Add(o *Stats) {
	s.InnerVisited += o.InnerVisited
	s.LeafRefs += o.LeafRefs
}
