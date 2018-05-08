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
}

func (n *node) removeKid(pos int) *node {
	if len(n.sublist) <= pos {
		return nil
	}
	r := n.sublist[pos]
	n.sublist = append(n.sublist[:pos], n.sublist[pos+1:]...)
	return r
}
