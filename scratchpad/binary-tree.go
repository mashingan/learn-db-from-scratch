package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
)

type nodeColor string

const (
	redNode   nodeColor = "red"
	blackNode nodeColor = "black"
)

const (
	printRed   = "\033[31m∎\033[0m"
	printBlack = "\033[107m\033[30m∎\033[107m \033[0m"
)

func (c nodeColor) String() string {
	if c == redNode {
		return printRed
	}
	return printBlack
}

type nodeTree struct {
	val                 int
	color               nodeColor
	left, right, parent *nodeTree
}

func (n *nodeTree) String() string {
	return fmt.Sprintf("val: %d, color: %s", n.val, n.color)
}

func (node *nodeTree) isLeaf() bool {
	if node.right == nil && node.left == nil {
		return true
	}
	return false
}

func (node *nodeTree) isBlack() bool {
	if node == nil || node.color == blackNode {
		return true
	}
	return false
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	r1, w1 := io.Pipe()
	go func(w io.WriteCloser) {
		defer w.Close()
		/*
			for i := 8; i >= 1; i-- {
				//for i := 1; i <= 8; i++ {
				w.Write([]byte(fmt.Sprintf("%d", i) + "\n"))
			}
		*/
		w.Write([]byte("8\n"))
		w.Write([]byte("1\n"))
		w.Write([]byte("7\n"))
		w.Write([]byte("2\n"))
		w.Write([]byte("5\n"))
		w.Write([]byte("3\n"))
		w.Write([]byte("4\n"))
		w.Write([]byte("6\n"))
		w.Write([]byte("9\n"))
		w.Write([]byte("10\n"))
		w.Write([]byte("11\n"))
		w.Write([]byte("12\n"))
		w.Write([]byte("13\n"))
		/*
		 */
		w.Write([]byte("q\n"))
	}(w1)
	s := bufio.NewScanner(r1)
	var (
		root *nodeTree = nil
	)
	for run, skip, node := scanning(s, &root); run; run, skip, node = scanning(s, &root) {
		if skip {
			continue
		}
		insertTree(root, node)
		balanceTree(&root, node)

	}
	log.Println("final tree")
	inspectTree(root, 0, "")

	n3 := searchTree(root, 3)
	log.Println("found tree 3:", n3, " whether it is leaf?", n3.isLeaf())
	log.Println("is tree valid?", validateTree(root))
	deleteNode(&root, 11)
	log.Println("after delete")
	inspectTree(root, 0, "")
	log.Println("is tree still valid?", validateTree(root))
}

func scanning(s *bufio.Scanner, root **nodeTree) (bool, bool, *nodeTree) {
	fmt.Print("> ")
	if !s.Scan() {
		return false, false, nil
	}
	line := s.Text()
	if line == "q" || line == "quit" {
		return false, false, nil
	}
	num, err := strconv.Atoi(line)
	if err != nil {
		log.Println(err)
		fmt.Println("type q or quit to break loop")
		return true, true, nil
	}
	if *root == nil {
		*root = &nodeTree{val: num, color: blackNode}
		return true, true, nil
	}
	node := &nodeTree{
		val: num, color: redNode,
	}
	return true, false, node
}

func insertTree(tree, node *nodeTree) {
	for tree != nil {
		if tree.val == node.val {
			break
		}
		if node.val < tree.val {
			if tree.left == nil {
				node.parent = tree
				tree.left = node
			}
			tree = tree.left
		} else if node.val > tree.val {
			if tree.right == nil {
				node.parent = tree
				tree.right = node
				break
			}
			tree = tree.right
		}
	}
}

func inspectTree(tree *nodeTree, deep int, dir string) {
	color := printRed
	if tree != nil {
		if tree.color == blackNode {
			color = printBlack
		}
		fmt.Printf("%stree val: %d %s %s\n", strings.Repeat(",", deep), tree.val, dir, color)
		inspectTree(tree.left, deep+1, "left")
		inspectTree(tree.right, deep+1, "right")
	} else {
		fmt.Printf("%stree nil\n", strings.Repeat(",", deep))
	}
}

func balanceTree(root **nodeTree, node *nodeTree) {
	parent := node.parent
	if parent == nil {
		if node != *root {
			*root = node
		}
		if node.color == redNode {
			node.color = blackNode
		}
		return
	}
	if parent.color == blackNode {
		return
	}
	uncle := parent.parent.right
	parentIsRight := false
	if parent == uncle {
		uncle = parent.parent.left
		parentIsRight = true
	}
	grandparent := parent.parent
	if node.color == redNode && parent.color == redNode /* property-4 */ {
		if uncle != nil && uncle.color == redNode {
			parent.color = blackNode
			uncle.color = blackNode
			grandparent.color = redNode
			balanceTree(root, grandparent)
			return
		}

		// zig-zag mode
		if parent.left == node && parentIsRight {
			rotateRight2(root, parent)
			grandparent.color = redNode
			grandparent.right.color = blackNode
			rotateLeft22(root, grandparent)
			// zig-zig rotate right at gp
		} else if parent.right == node && !parentIsRight {
			rotateLeft22(root, parent)
			grandparent.color = redNode
			grandparent.left.color = blackNode
			rotateRight2(root, grandparent)
			// zig-zig rotate right at gp
		} else if parent.left == node && !parentIsRight {
			grandparent.color = redNode
			grandparent.left.color = blackNode
			rotateRight2(root, grandparent)

		} else if parent.right == node && parentIsRight {
			grandparent.color = redNode
			grandparent.right.color = blackNode
			rotateLeft22(root, grandparent)
		}
	}
}

func rotateRight2(root **nodeTree, node *nodeTree) {
	left := node.left
	parent := node.parent
	if parent == nil {
		*root = left
	} else {
		if parent.right == node {
			parent.right = left
		} else {
			parent.left = left
		}
	}
	left.parent = parent
	node.parent = left
	node.left = left.right
	if node.left != nil {
		node.left.parent = node
	}
	left.right = node
}

func rotateLeft22(root **nodeTree, node *nodeTree) {
	right := node.right
	parent := node.parent
	if parent == nil {
		*root = right
	} else {
		if parent.right == node {
			parent.right = right
		} else {
			parent.left = right
		}
	}
	right.parent = parent
	node.parent = right
	node.right = right.left
	if node.right != nil {
		node.right.parent = node
	}
	right.left = node
}

func searchTree(root *nodeTree, n int) *nodeTree {
	tree := root
	for tree != nil {
		if n == tree.val {
			return tree
		}
		if n < tree.val {
			tree = tree.left
			continue
		}
		tree = tree.right
	}
	return tree
}

func deleteNode(root **nodeTree, n int) {
	node := searchTree(*root, n)
	if node == nil {
		return
	}

	if node.isLeaf() {
		if !node.isBlack() {
			if node.parent.right == node {
				node.parent.right = nil
			} else {
				node.parent.left = nil
			}
			return
		}
		siblingFixup(root, node, node, node.parent)
		return
	}

	var (
		childrenRight = node.right
		crParent      *nodeTree
		childrenLeft  = node.left
		clParent      *nodeTree
		wg            sync.WaitGroup
	)
	wg.Add(2)
	go func(c, p **nodeTree, w *sync.WaitGroup) {
		defer w.Done()
		*c = node.right
		for *c != nil && (*c).left != nil {
			*c = (*c).left
		}
		if *c == nil {
			return
		}
		*p = (*c).parent
		(*c).parent = nil
	}(&childrenRight, &crParent, &wg)
	go func(c, p **nodeTree, w *sync.WaitGroup) {
		defer w.Done()
		*c = node.left
		for *c != nil && (*c).right != nil {
			*c = (*c).right
		}
		if *c == nil {
			return
		}
		*p = (*c).parent
		(*c).parent = nil
	}(&childrenLeft, &clParent, &wg)
	wg.Wait()
	if childrenLeft != nil && childrenLeft.color == redNode {
		node.val = childrenLeft.val
		if clParent == node {
			clParent.left = nil
		} else if clParent != nil {
			clParent.right = nil
		}
		return
	} else if childrenRight != nil && childrenRight.color == redNode {
		node.val = childrenRight.val
		if crParent == node {
			crParent.right = nil
		} else if crParent != nil {
			crParent.left = nil
		}
		return
	}
	if child := childrenLeft.left; child != nil && !child.isBlack() {
		node.val = childrenLeft.val
		if node.left != childrenLeft {
			clParent.right = child
		} else {
			clParent.left = child
		}
		child.color = blackNode
		child.parent = clParent
		childrenLeft = nil
		return
	} else if child := childrenRight.right; child != nil && !child.isBlack() {
		node.val = childrenRight.val
		if node.right != childrenRight {
			crParent.left = child
		} else {
			crParent.right = child
		}
		child.color = blackNode
		child.parent = crParent
		childrenRight = nil
		return
	}
	siblingFixup(root, node, childrenRight, crParent)
}

func validateTree(node *nodeTree) bool {
	if node == nil {
		return true
	}
	_, validLeft := heightBlack(node.left)
	_, validRight := heightBlack(node.right)
	return validLeft == validRight
}

func heightBlack(node *nodeTree) (int, bool) {
	if node == nil {
		return 1, true
	}
	count := 0
	if node.color == blackNode {
		count++
	}
	leftHeight, vl := heightBlack(node.left)
	rightHeight, vr := heightBlack(node.right)
	if leftHeight != rightHeight || !vl || !vr {
		return 0, false
	}
	return count + leftHeight, true
}

func siblingFixup(root **nodeTree, node, childrenRight, crParent *nodeTree) {
	sr := crParent.right
	direct := false
	if childrenRight == node.right {
		sr = node.left
		node.right = nil
		direct = true
	} else if childrenRight == node {
		if crParent.left != node {
			// specific case where direct child at left
			sr = crParent.left
			crParent.right = nil
			direct = true
		} else {
			crParent.left = nil
		}
	}
	if sr == nil {
		return
	}

	if childrenRight != node {
		node.val = childrenRight.val
	}
	childrenRight.parent = nil
	if !sr.isBlack() {
		rotateRight2(root, crParent)
		crParent.color = redNode
		sr.color = blackNode
		return
	}

	if crParent.right == sr {
		crParent.left = nil
	} else {
		crParent.right = nil
	}

	if sr.left.isBlack() && sr.right.isBlack() {
		for crParent != *root {
			sr.color = redNode
			if !crParent.isBlack() {
				crParent.color = blackNode
				return
			}
			if crParent.right == sr {
				sr = crParent.parent.left
			} else {
				sr = crParent.parent.right
			}
			crParent = sr.parent
		}
		return
	}

	var (
		farchild  = sr.right
		nearchild = sr.left
	)
	if direct {
		farchild = sr.left
		nearchild = sr.right
	}

	if !farchild.isBlack() {
		if direct {
			rotateRight2(root, crParent)
		} else {
			rotateLeft22(root, crParent)
		}
		log.Printf("sr: %s, crParent: %s, farchild: %s\n", sr, crParent, farchild)
		sr.color = crParent.color
		crParent.color = blackNode
		farchild.color = blackNode
		return
	}

	if farchild.isBlack() && !nearchild.isBlack() {
		if direct {
			rotateLeft22(root, sr)
			rotateRight2(root, crParent)
		} else {
			rotateRight2(root, sr)
			rotateLeft22(root, crParent)
		}
		log.Printf("sr: %s, crParent: %s, farchild: %s\n", sr, crParent, farchild)
		sr.color = blackNode
		crParent.color = blackNode
		return
	}
}
