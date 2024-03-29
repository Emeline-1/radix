package radix

import (
	"sort"
	"strings"
	"errors"
	"fmt"
	"net"
	"strconv"
	"github.com/nlpodyssey/gopickle/types"
)

// WalkFn is used when walking the tree. Takes a
// key and value, returning if iteration should
// be terminated.
type WalkFn func(s string, v interface{}) bool

// WalkFnPost is used when walking the tree in post order.
// It takes a parent and its children as argument.
type WalkFnPost func (parent *LeafNode, children []*LeafNode)

// leafNode is used to represent a value
type LeafNode struct {
	Key string
	Val interface{}
}

// edge is used to represent an edge node
type edge struct {
	label byte
	node  *node
}

type node struct {
	// leaf is used to store possible leaf
	leaf *LeafNode

	// prefix is the common prefix we ignore
	prefix string

	// Edges should be stored in-order for iteration.
	// We avoid a fully materialized slice to save memory,
	// since in most cases we expect to be sparse
	edges edges
}

func (node *node) Call(args ...interface{}) (interface{}, error) {
	return node, nil
}

func (n *node) isLeaf() bool {
	return n.leaf != nil
}

func (n *node) addEdge(e edge) {
	n.edges = append(n.edges, e)
	n.edges.Sort()
}

func (n *node) updateEdge(label byte, node *node) {
	num := len(n.edges)
	idx := sort.Search(num, func(i int) bool {
		return n.edges[i].label >= label
	})
	if idx < num && n.edges[idx].label == label {
		n.edges[idx].node = node
		return
	}
	panic("replacing missing edge")
}

func (n *node) getEdge(label byte) *node {
	num := len(n.edges)
	idx := sort.Search(num, func(i int) bool {
		return n.edges[i].label >= label
	})
	if idx < num && n.edges[idx].label == label {
		return n.edges[idx].node
	}
	return nil
}

func (n *node) delEdge(label byte) {
	num := len(n.edges)
	idx := sort.Search(num, func(i int) bool {
		return n.edges[i].label >= label
	})
	if idx < num && n.edges[idx].label == label {
		copy(n.edges[idx:], n.edges[idx+1:])
		n.edges[len(n.edges)-1] = edge{}
		n.edges = n.edges[:len(n.edges)-1]
	}
}

type edges []edge

func (e edges) Len() int {
	return len(e)
}

func (e edges) Less(i, j int) bool {
	return e[i].label < e[j].label
}

func (e edges) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func (e edges) Sort() {
	sort.Sort(e)
}

// Tree implements a radix tree. This can be treated as a
// Dictionary abstract data type. The main advantage over
// a standard hash map is prefix-based lookups and
// ordered iteration,
type Tree struct {
	root *node
	size int
}

func (tree *Tree) String () string {
	return "Not implemented on purpose"
}

// New returns an empty Tree
func New() *Tree {
	return NewFromMap(nil)
}

//For pickle
func (tree *Tree) Call(args ...interface{}) (interface{}, error) {
	return tree, nil
}

// For pickle
func (tree *Tree) PySetState(state interface{}) error {
	elements, ok := state.(*types.List); 
	if !ok {
		return errors.New (fmt.Sprintf ("Radix.tree: Expected *types.List but got %T", state))
	}

	for _, element_i := range *elements {
		/* --- Access all data --- */
		element, t1 := element_i.(*types.Tuple)
		if !t1 {
			return errors.New (fmt.Sprintf ("Radix.tree: Expected *types.Tuple but got %T", element_i))
		}

		prefix, t2 := element.Get(0).(string)
		if !t2 {
			return errors.New (fmt.Sprintf ("Radix.tree: Expected string but got %T", element.Get(0)))
		}
		if strings.Contains (prefix, ":") { // Ignore IPv6
			continue
		}
		as_dict, t3 := element.Get(1).(*types.Dict)
		if !t3 {
			return errors.New (fmt.Sprintf ("Radix.tree: Expected *types.Dict: %T", element.Get(1)))
		}

		d, present := as_dict.Get ("as")
		if !present {
			return errors.New (fmt.Sprintf ("Radix.tree: no 'as' key present"))
		}

		/* --- Insert prefix in tree --- */
    	radix_prefix := get_binary_string (prefix)
        tree.Insert (radix_prefix, d)
	}
	return nil
}

// For pickle
func FindClass (module, name string) (interface{}, error) {
    if module == "radix" && name == "Radix" {
        return New(), nil
    }
    return nil, fmt.Errorf("class not found :( " + module + " " + name)
}

// NewFromMap returns a new tree containing the keys
// from an existing map
func NewFromMap(m map[string]interface{}) *Tree {
	t := &Tree{root: &node{}}
	for k, v := range m {
		t.Insert(k, v)
	}
	return t
}

// Len is used to return the number of elements in the tree
func (t *Tree) Len() int {
	return t.size
}

// longestPrefix finds the length of the shared prefix
// of two strings
func longestPrefix(k1, k2 string) int {
	max := len(k1)
	if l := len(k2); l < max {
		max = l
	}
	var i int
	for i = 0; i < max; i++ {
		if k1[i] != k2[i] {
			break
		}
	}
	return i
}

/**
 * Returns the prefix as a binary string.
 * The binary string is cut at mask length.
 * ex: 1.0.4.0/22 -> "0000000100000000000001"
 */
func get_binary_string (prefix string) string {

    ip := strings.Split (prefix, "/")[0]
    ip_byte := net.ParseIP (ip)

    var ip_string string
    if len (ip_byte) == 4 {
        ip_string = fmt.Sprintf("%08b%08b%08b%08b", ip_byte[0], ip_byte[1], ip_byte[2], ip_byte[3])
    } else {
        ip_string = fmt.Sprintf("%08b%08b%08b%08b", ip_byte[12], ip_byte[13], ip_byte[14], ip_byte[15])
    }
    
    l,_ := strconv.Atoi (strings.Split (prefix, "/")[1])
    return ip_string[:l]
}

// Insert is used to add a newentry or update
// an existing entry. Returns if updated.
func (t *Tree) Insert(s string, v interface{}) (interface{}, bool) {
	var parent *node
	n := t.root
	search := s
	for {
		// Handle key exhaution
		if len(search) == 0 {
			if n.isLeaf() {
				old := n.leaf.Val
				n.leaf.Val = v
				return old, true
			}

			n.leaf = &LeafNode{
				Key: s,
				Val: v,
			}
			t.size++
			return nil, false
		}

		// Look for the edge
		parent = n
		n = n.getEdge(search[0])

		// No edge, create one
		if n == nil {
			e := edge{
				label: search[0],
				node: &node{
					leaf: &LeafNode{
						Key: s,
						Val: v,
					},
					prefix: search,
				},
			}
			parent.addEdge(e)
			t.size++
			return nil, false
		}

		// Determine longest prefix of the search key on match
		commonPrefix := longestPrefix(search, n.prefix)
		if commonPrefix == len(n.prefix) {
			search = search[commonPrefix:]
			continue
		}

		// Split the node
		t.size++
		child := &node{
			prefix: search[:commonPrefix],
		}
		parent.updateEdge(search[0], child)

		// Restore the existing node
		child.addEdge(edge{
			label: n.prefix[commonPrefix],
			node:  n,
		})
		n.prefix = n.prefix[commonPrefix:]

		// Create a new leaf node
		leaf := &LeafNode{
			Key: s,
			Val: v,
		}

		// If the new key is a subset, add to to this node
		search = search[commonPrefix:]
		if len(search) == 0 {
			child.leaf = leaf
			return nil, false
		}

		// Create a new edge for the node
		child.addEdge(edge{
			label: search[0],
			node: &node{
				leaf:   leaf,
				prefix: search,
			},
		})
		return nil, false
	}
}

// Delete is used to delete a key, returning the previous
// value and if it was deleted
func (t *Tree) Delete(s string) (interface{}, bool) {
	var parent *node
	var label byte
	n := t.root
	search := s
	for {
		// Check for key exhaution
		if len(search) == 0 {
			if !n.isLeaf() {
				break
			}
			goto DELETE
		}

		// Look for an edge
		parent = n
		label = search[0]
		n = n.getEdge(label)
		if n == nil {
			break
		}

		// Consume the search prefix
		if strings.HasPrefix(search, n.prefix) {
			search = search[len(n.prefix):]
		} else {
			break
		}
	}
	return nil, false

DELETE:
	// Delete the leaf
	leaf := n.leaf
	n.leaf = nil
	t.size--

	// Check if we should delete this node from the parent
	if parent != nil && len(n.edges) == 0 {
		parent.delEdge(label)
	}

	// Check if we should merge this node
	if n != t.root && len(n.edges) == 1 {
		n.mergeChild()
	}

	// Check if we should merge the parent's other child
	if parent != nil && parent != t.root && len(parent.edges) == 1 && !parent.isLeaf() {
		parent.mergeChild()
	}

	return leaf.Val, true
}

// DeletePrefix is used to delete the subtree under a prefix
// Returns how many nodes were deleted
// Use this to delete large subtrees efficiently
func (t *Tree) DeletePrefix(s string) int {
	return t.deletePrefix(nil, t.root, s)
}

// delete does a recursive deletion
func (t *Tree) deletePrefix(parent, n *node, prefix string) int {
	// Check for key exhaustion
	if len(prefix) == 0 {
		// Remove the leaf node
		subTreeSize := 0
		//recursively walk from all edges of the node to be deleted
		recursiveWalk(n, func(s string, v interface{}) bool {
			subTreeSize++
			return false
		})
		if n.isLeaf() {
			n.leaf = nil
		}
		n.edges = nil // deletes the entire subtree

		// Check if we should merge the parent's other child
		if parent != nil && parent != t.root && len(parent.edges) == 1 && !parent.isLeaf() {
			parent.mergeChild()
		}
		t.size -= subTreeSize
		return subTreeSize
	}

	// Look for an edge
	label := prefix[0]
	child := n.getEdge(label)
	if child == nil || (!strings.HasPrefix(child.prefix, prefix) && !strings.HasPrefix(prefix, child.prefix)) {
		return 0
	}

	// Consume the search prefix
	if len(child.prefix) > len(prefix) {
		prefix = prefix[len(prefix):]
	} else {
		prefix = prefix[len(child.prefix):]
	}
	return t.deletePrefix(n, child, prefix)
}

func (n *node) mergeChild() {
	e := n.edges[0]
	child := e.node
	n.prefix = n.prefix + child.prefix
	n.leaf = child.leaf
	n.edges = child.edges
}

// Get is used to lookup a specific key, returning
// the value and if it was found
func (t *Tree) Get(s string) (interface{}, bool) {
	n := t.root
	search := s
	for {
		// Check for key exhaution
		if len(search) == 0 {
			if n.isLeaf() {
				return n.leaf.Val, true
			}
			break
		}

		// Look for an edge
		n = n.getEdge(search[0])
		if n == nil {
			break
		}

		// Consume the search prefix
		if strings.HasPrefix(search, n.prefix) {
			search = search[len(n.prefix):]
		} else {
			break
		}
	}
	return nil, false
}

func (t *Tree) LongestPrefix(s string) (key string, val interface{}, pres bool) {
	key, val, pres, _ = t.longestPrefix (s)
	return
}

// LongestPrefix is like Get, but instead of an
// exact match, it will return the longest prefix match.
// When looking for a longest prefix, remove last bit.
func (t *Tree) longestPrefix(s string) (string, interface{}, bool, *node) {
	var last *node
	n := t.root
	search := s
	for {
		// Look for a leaf node
		if n.isLeaf() {
			last = n
		}

		// Check for key exhaution
		if len(search) == 0 {
			break
		}

		// Look for an edge
		n = n.getEdge(search[0])
		if n == nil {
			break
		}

		// Consume the search prefix
		if strings.HasPrefix(search, n.prefix) {
			search = search[len(n.prefix):]
		} else {
			break
		}
	}
	if last != nil {
		return last.leaf.Key, last.leaf.Val, true, last
	}
	return "", nil, false, nil
}

func (t *Tree) BFS_print () {
	/* --- BFS traversal --- */
	queue := make ([]*node, 0)
	queue = append (queue, t.root)
	for len (queue) != 0 {
		curr := queue[0]
		queue = queue[1:]

		fmt.Println ("-----------")
		fmt.Println ("Node prefix", curr.prefix)
		if curr.isLeaf () {
			fmt.Println ("Node leaf", curr.leaf.Key)
		} else {
			fmt.Println ("Node leaf: /")
		}

		for _, edge := range (curr.edges) {
			queue = append (queue, edge.node)
			fmt.Println ("Edges:", string(edge.label))
		}
	}
}

/**
 * Given a prefix A, this function returns the first prefix equal or included
 * in A, but not included or equal to any of A's more specifics. 
 * 
 * A node can have:
 * - 0 children: there is no more-specific prefix included in prefix A.
 *   This means that any address can be picked within prefix A without worry. Return prefix A.
 * - 1 child: there exist one or more more-specific prefixes within A, but there is room for more.
 *   Return the first non-included prefix.
 * - 2 children: prefix A is entirely covered by its more-specific prefixes. There is no room
 *   left to pick an IP address. Return false.
 * 
 * By construction of the radix tree, a BFS is not necessary to find
 * the first non included prefix. Knowing how many children a node has
 * is enough to determine in which cas we are.
 */
func (t *Tree) FirstNonIncludedPrefix (s string) (string, bool) {
	_,_, pres, n := t.longestPrefix (s)
	if pres {
		if len (n.edges) == 0 { // Node has no children -> Return self
			return n.leaf.Key, true
		}

		if len (n.edges) == 1 { // Node has 1 child -> Return the counterpart child
			return n.leaf.Key + invert_binary_label (n.edges[0].label), true
		}
		// Node has 2 children -> Prefix s is completely covered.
	}
	return "", false
}

func invert_binary_label (s byte) string {
	if string (s) == "1" {
		return "0"
	} else {
		return "1"
	}
}

// Minimum is used to return the minimum value in the tree
func (t *Tree) Minimum() (string, interface{}, bool) {
	n := t.root
	for {
		if n.isLeaf() {
			return n.leaf.Key, n.leaf.Val, true
		}
		if len(n.edges) > 0 {
			n = n.edges[0].node
		} else {
			break
		}
	}
	return "", nil, false
}

// Maximum is used to return the maximum value in the tree
func (t *Tree) Maximum() (string, interface{}, bool) {
	n := t.root
	for {
		if num := len(n.edges); num > 0 {
			n = n.edges[num-1].node
			continue
		}
		if n.isLeaf() {
			return n.leaf.Key, n.leaf.Val, true
		}
		break
	}
	return "", nil, false
}

// Walk is used to walk the tree
func (t *Tree) Walk(fn WalkFn) {
	recursiveWalk(t.root, fn)
}

// WalkPrefix is used to walk the tree under a prefix
func (t *Tree) WalkPrefix(prefix string, fn WalkFn) {
	n := t.root
	search := prefix
	for {
		// Check for key exhaution
		if len(search) == 0 {
			recursiveWalk(n, fn)
			return
		}

		// Look for an edge
		n = n.getEdge(search[0])
		if n == nil {
			break
		}

		// Consume the search prefix
		if strings.HasPrefix(search, n.prefix) {
			search = search[len(n.prefix):]

		} else if strings.HasPrefix(n.prefix, search) {
			// Child may be under our search prefix
			recursiveWalk(n, fn)
			return
		} else {
			break
		}
	}

}

// WalkPath is used to walk the tree, but only visiting nodes
// from the root down to a given leaf. Where WalkPrefix walks
// all the entries *under* the given prefix, this walks the
// entries *above* the given prefix.
func (t *Tree) WalkPath(path string, fn WalkFn) {
	n := t.root
	search := path
	for {
		// Visit the leaf values if any
		if n.leaf != nil && fn(n.leaf.Key, n.leaf.Val) {
			return
		}

		// Check for key exhaution
		if len(search) == 0 {
			return
		}

		// Look for an edge
		n = n.getEdge(search[0])
		if n == nil {
			return
		}

		// Consume the search prefix
		if strings.HasPrefix(search, n.prefix) {
			search = search[len(n.prefix):]
		} else {
			break
		}
	}
}

// recursiveWalk is used to do a pre-order walk of a node
// recursively. Returns true if the walk should be aborted
func recursiveWalk(n *node, fn WalkFn) bool {
	// Visit the leaf values if any
	if n.leaf != nil && fn(n.leaf.Key, n.leaf.Val) {
		return true
	}

	// Recurse on the children
	for _, e := range n.edges {
		if recursiveWalk(e.node, fn) {
			return true
		}
	}
	return false
}

// Walk_post is used to walk the tree in post order
func (t *Tree) Walk_post(fn WalkFnPost) {
	recursiveWalk_postOrder(t.root, fn)
}

// recursiveWalk_postOrder is used to do a post-order walk
// of a node recursively. 
// Returns the node itself, or its children if the node
// was an intermediate node.
func recursiveWalk_postOrder (n *node, fn WalkFnPost) []*LeafNode{

	//Visit children
	all_children := make ([]*LeafNode, 0, len (n.edges))
	for _, e := range n.edges {
		children := recursiveWalk_postOrder (e.node, fn)
		all_children = append (all_children, children...)
	}

	if n.leaf == nil { 
		// Intermediate node (doesn't correspond to a key that was inserted in the tree) -> Return your children and not yourself
		// Do not call user processing function
		return all_children
	}

	// Here, you are not an intermediate node, and you have all your children values, whether direct or not.
	if n.leaf != nil {
		fn (n.leaf, all_children)
	}

	return []*LeafNode {n.leaf} // You have processed your children, return yourself to your parent.
}

// ToMap is used to walk the tree and convert it into a map
func (t *Tree) ToMap(a func (string) string) map[string]interface{} {
	out := make(map[string]interface{}, t.size)
	t.Walk(func(k string, v interface{}) bool {
		out[a(k)] = v
		return false
	})
	return out
}