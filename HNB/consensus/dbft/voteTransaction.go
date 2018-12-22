package dbft

type VoteInfo struct {
	TxId        string `json:"tx_id"`
	AccountId   int64  `json:"account_id"`
	PeerId      string `json:"peer_id"`
	VotingPower int64  `json:"voting_power"`
	VoteEpoch   int64  `json:"vote_epoch"`
	InTime      uint64 `json:"in_time"`
}
