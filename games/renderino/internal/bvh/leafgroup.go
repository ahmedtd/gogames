package bvh

// makeLeafGroup builds a leaf gathering the given elements, with bounds that
// are the union of theirs, and appends their refs to t.LeafGroups.  The
// element nodes themselves are not part of the tree.
func (t *Tree) makeLeafGroup(elements []*Node) *Node {
	refs := make([]int, len(elements))
	lo, hi := elements[0].Lo, elements[0].Hi
	for i, e := range elements {
		refs[i] = e.Ref
		for k := range 3 {
			lo[k] = min(lo[k], e.Lo[k])
			hi[k] = max(hi[k], e.Hi[k])
		}
	}

	node := t.GetFreeNode()
	node.Type = NodeTypeConcrete
	node.Lo, node.Hi = lo, hi
	node.Ref = len(t.LeafGroups)
	t.LeafGroups = append(t.LeafGroups, refs)
	return node
}
