package merkle

import (
	"testing"
	"fmt"
	"encoding/hex"
	"crypto/sha256"
	"strings"
)


func TestMerkleRoot(t *testing.T) {
	a := make([]BYTE, 0)
	for i := 0; i < 10; i++ {
		tmp := fmt.Sprintf("%d", i)
		hash := sha256.New()
		hash.Write([]byte(tmp))
		temp1 := hash.Sum(nil)
		a = append(a, BYTE(temp1))
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


func printMerkle(mt *merkleTreeNode, level uint)  {
	//if mt == nil {
	//	fmt.Println("level", level)
	//	return
	//}
	if mt.Right == nil || mt.Left == nil {
		fmt.Println("level", level, hex.EncodeToString(mt.Hash))
		return
	}
	lala := strings.Repeat(" ", int(level*2))
	fmt.Println(lala, "level", level, hex.EncodeToString(mt.Hash))
	printMerkle(mt.Left, level-1)
	printMerkle(mt.Right, level-1)
}
