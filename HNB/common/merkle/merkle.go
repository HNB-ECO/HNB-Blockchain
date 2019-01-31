package merkle

import (
	"bytes"
	"crypto/sha256"
	"errors"
	//"fmt"
)

type BYTE []byte

func doubleSha256(s []BYTE) BYTE {
	b := new(bytes.Buffer)
	for _, d := range s {
		b.Write([]byte(d))
	}
	hash := sha256.New()
	hash.Write(b.Bytes())
	temp := hash.Sum(nil)
	hash1 := sha256.New()
	hash1.Write(temp)
	return BYTE(hash1.Sum(nil))
}

type merkleTree struct {
	Depth uint
	Root  *merkleTreeNode
}

type merkleTreeNode struct {
	Hash  BYTE
	Left  *merkleTreeNode
	Right *merkleTreeNode
}

func (t *merkleTreeNode) IsLeaf() bool {
	return t.Left == nil && t.Right == nil
}

//use []BYTE to create a new merkleTree
func newMerkleTree(hashes []BYTE) (*merkleTree, error) {
	if len(hashes) == 0 {
		return nil, errors.New("NewMerkleTree input no item error.")
	}
	var height uint

	height = 1
	nodes := generateLeaves(hashes)
	for len(nodes) > 1 {
		nodes = levelUp(nodes)
		height += 1
	}
	//fmt.Printf("the node len is %d\n", len(nodes))
	mt := &merkleTree{
		Root:  nodes[0],
		Depth: height,
	}
	return mt, nil

}

//Generate the leaves nodes
func generateLeaves(hashes []BYTE) []*merkleTreeNode {
	var leaves []*merkleTreeNode
	for _, d := range hashes {
		node := &merkleTreeNode{
			Hash: d,
		}
		leaves = append(leaves, node)
	}
	return leaves
}

//calc the next level's hash use double sha256
func levelUp(nodes []*merkleTreeNode) []*merkleTreeNode {
	var nextLevel []*merkleTreeNode
	for i := 0; i < len(nodes)/2; i++ {
		var data []BYTE
		data = append(data, nodes[i*2].Hash)
		data = append(data, nodes[i*2+1].Hash)
		hash := doubleSha256(data)
		node := &merkleTreeNode{
			Hash:  hash,
			Left:  nodes[i*2],
			Right: nodes[i*2+1],
		}
		nextLevel = append(nextLevel, node)
	}
	if len(nodes)%2 == 1 {
		var data []BYTE
		data = append(data, nodes[len(nodes)-1].Hash)
		data = append(data, nodes[len(nodes)-1].Hash)
		hash := doubleSha256(data)
		node := &merkleTreeNode{
			Hash:  hash,
			Left:  nodes[len(nodes)-1],
			Right: nodes[len(nodes)-1],
		}
		nextLevel = append(nextLevel, node)
	}
	return nextLevel
}
