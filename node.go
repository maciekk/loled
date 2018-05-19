package main

// The underlying model for all data in this program, these list of lists of
// lists of ..., is essentially a tree. Here, a node is an element in that
// tree.
type node struct {
	// node content payload
	label string
	// parent node; nil if no parent (should be true only for root node)
	parent *node
	// list of children
	sublist []*node
	// Is it tagged?
	tagged bool
}

func (n *node) insertKid(pos int, newkid *node) {
	n.sublist = append(n.sublist, nil) // extend length by 1
	copy(n.sublist[pos+1:], n.sublist[pos:])
	n.sublist[pos] = newkid
	newkid.parent = n
}

func (n *node) removeKid(pos int) *node {
	if len(n.sublist) <= pos {
		return nil
	}
	r := n.sublist[pos]
	n.sublist = append(n.sublist[:pos], n.sublist[pos+1:]...)
	r.parent = nil
	return r
}

// Returns number of nodes in tree, and its max depth.
func (n *node) Analyze() (int, int) {
	// Start off by counting self.
	count := 1
	depth := 1
	for _, kid := range n.sublist {
		kid_count, kid_depth := kid.Analyze()
		count += kid_count
		depth = max(depth, kid_depth+1)
	}
	return count, depth
}

// vim: fdm=syntax
