package bvh

// makeLeafGroup builds a leaf gathering the given elements, with bounds that
// are the union of theirs, and appends their refs to t.LeafGroups.
func (t *Tree) makeLeafGroup(elements []RawElement) *Node {
	refs := make([]int, len(elements))
	lo, hi := elements[0].Bounds.Lo, elements[0].Bounds.Hi
	for i, e := range elements {
		refs[i] = e.Ref
		for k := range 3 {
			lo[k] = min(lo[k], e.Bounds.Lo[k])
			hi[k] = max(hi[k], e.Bounds.Hi[k])
		}
	}

	node := t.getFreeNode()
	node.Type = NodeTypeConcrete
	node.Lo, node.Hi = lo, hi
	node.Ref = len(t.LeafGroups)
	t.LeafGroups = append(t.LeafGroups, refs)
	return node
}
