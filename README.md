# radix

This project implements a [radix tree](https://en.wikipedia.org/wiki/Radix_tree) in the [Go Language](https://golang.org). 
The code is taken from [armon's repository](https://github.com/armon/go-radix), with a few modifications. 

The main modification is the addition of the possibility to walk the tree in post order.

#### Example
```
import (radix "github.com/Emeline-1/radix")

tree := radix.New()
tree.Insert ("100010", "data")
tree.Insert ("1000101", "data")
tree.Insert ("1000100", "data")

tree.Walk_post (walk_radix_tree)

func walk_radix_tree (parent *radix.LeafNode, children []*radix.LeafNode) {
  // Do something with the parent and its children
}
```
