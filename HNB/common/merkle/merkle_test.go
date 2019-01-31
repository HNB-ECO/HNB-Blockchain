package merkle

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestMerkleRoot(t *testing.T) {
	a := make([]BYTE, 0)
	for i := 0; i < 9; i++ {
		tmp := fmt.Sprintf("%d", i)
		a = append(a, BYTE(tmp))
	}
	b := ComputeMerkleRoot(a)
	if b == nil {
		fmt.Println("lalal")
	} else {
		printMerkle(b.Root, b.Depth)
	}
}
func ComputeMerkleRoot(hashes []BYTE) *merkleTree {
	if len(hashes) == 0 {
		return nil
	}
	tree, _ := newMerkleTree(hashes)
	return tree
}

func printMerkle(mt *merkleTreeNode, level uint) {
	//if mt == nil {
	//	fmt.Println("level", level)
	//	return
	//}
	if mt.Right == nil || mt.Left == nil {
		fmt.Println("level", level, hex.EncodeToString(mt.Hash))
		return
	}
	fmt.Println("level", level, hex.EncodeToString(mt.Hash))
	printMerkle(mt.Left, level-1)
	printMerkle(mt.Right, level-1)
}
