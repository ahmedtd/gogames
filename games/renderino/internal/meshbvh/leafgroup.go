package meshbvh

// makeLeafGroup builds a leaf gathering the elements located by group, with
// bounds that are the union of theirs, and appends group to t.LeafGroups.
func (t *Tree) makeLeafGroup(group LeafGroup) *Node {
	elements := t.Elements[group.First : group.First+group.Count]
	lo, hi := elements[0].Bounds.Lo, elements[0].Bounds.Hi
	for _, e := range elements[1:] {
		for k := range 3 {
			lo[k] = min(lo[k], e.Bounds.Lo[k])
			hi[k] = max(hi[k], e.Bounds.Hi[k])
		}
	}

	node := t.getFreeNode()
	node.Type = NodeTypeConcrete
	node.Lo, node.Hi = lo, hi
	node.Ref = len(t.LeafGroups)
	t.LeafGroups = append(t.LeafGroups, group)
	return node
}
