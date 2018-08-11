package merkle

import (
	"testing"
	"fmt"
)


func TestMerkleRoot(t *testing.T) {
	a := make([]BYTE, 0)
	for i := 0; i < 10; i++ {
		tmp := fmt.Sprintf("%d", i)
		a = append(a, BYTE(tmp))
	}
	b := ComputeMerkleRoot(a)
	if b == nil {
		fmt.Println("lalal")
	} else {
		print(b.Depth)
	}
}
func ComputeMerkleRoot(hashes []BYTE) *merkleTree {
	if len(hashes) == 0 {
		return nil
	}
	tree, _ := newMerkleTree(hashes)
	return tree
}
