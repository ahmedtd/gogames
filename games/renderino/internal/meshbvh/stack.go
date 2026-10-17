package meshbvh

// rayTestEntry is a pending node (an index into Tree.nodes) on a query's work
// stack, along with the ray parameter at which the ray enters its bounds.
type rayTestEntry struct {
	node   int32
	tEnter float32
}

// Stack is scratch space for a query's traversal.  Reusing one across calls
// saves allocating and zeroing a fresh work stack on every call.
//
// A Stack may be shared by nested calls (a FrustumTest callback that itself
// queries a tree): each call pushes its work on top of the enclosing call's,
// and pops it all before returning.  It must not be shared by concurrent
// calls, so each thread needs its own.
type Stack struct {
	entries []rayTestEntry
}
