package common

type StateSet struct {
	SI []*StateItem
}

type ItemState byte

const (
	None    ItemState = iota //no change
	Changed                  //which was be mark delete
	Deleted                  //which wad be mark delete
)

//State item struct
type StateItem struct {
	Key     []byte    //State key
	Value   []byte    //State value
	State   ItemState //Status
	Trie    bool      //no use
	ChainID string
}

type StateStore interface {
	GetState(chainID string, key []byte) ([]byte, error)
	SetState(chainID string, key []byte, state []byte) error
	DeleteState(chainID string, key []byte) error
}
