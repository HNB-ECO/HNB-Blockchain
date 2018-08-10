
package peer

import (
	"fmt"
	"sync"
	"HNB/p2pNetwork/common"
	"HNB/p2pNetwork/message/bean"
)

type NbrPeers struct {
	sync.RWMutex
	List map[uint64]*Peer
}

func (np *NbrPeers) Broadcast(msg bean.Message, isConsensus bool) {
	np.RLock()
	defer np.RUnlock()
	for _, node := range np.List {
		if node.syncState == common.ESTABLISH && node.GetRelay() == true {
			err := node.Send(msg, isConsensus)
			if err != nil{
				P2PLog.Info(LOGTABLE_NETWORK, "send msg err: " + err.Error())
			}
		}
	}
}

func (np *NbrPeers) NodeExisted(uid uint64) bool {
	_, ok := np.List[uid]
	return ok
}

func (np *NbrPeers) GetPeer(id uint64) *Peer {
	np.Lock()
	defer np.Unlock()
	n, ok := np.List[id]
	if ok == false {
		return nil
	}
	return n
}

func (np *NbrPeers) AddNbrNode(p *Peer) {
	np.Lock()
	defer np.Unlock()

	if np.NodeExisted(p.GetID()) {
		fmt.Printf("insert an existed node\n")
	} else {
		np.List[p.GetID()] = p
	}
}

func (np *NbrPeers) DelNbrNode(id uint64) (*Peer, bool) {
	np.Lock()
	defer np.Unlock()

	n, ok := np.List[id]
	if ok == false {
		return nil, false
	}
	delete(np.List, id)
	return n, true
}

func (np *NbrPeers) Init() {
	np.List = make(map[uint64]*Peer)
}

func (np *NbrPeers) NodeEstablished(id uint64) bool {
	np.RLock()
	defer np.RUnlock()

	n, ok := np.List[id]
	if ok == false {
		return false
	}

	if n.syncState != common.ESTABLISH {
		return false
	}

	return true
}

func (np *NbrPeers) GetNeighborAddrs() []common.PeerAddr {
	np.RLock()
	defer np.RUnlock()

	var addrs []common.PeerAddr
	for _, p := range np.List {
		if p.GetSyncState() != common.ESTABLISH {
			continue
		}
		var addr common.PeerAddr
		addr.IpAddr, _ = p.GetAddr16()
		addr.Time = p.GetTimeStamp()
		addr.Services = p.GetServices()
		addr.Port = p.GetSyncPort()
		addr.ID = p.GetID()
		addrs = append(addrs, addr)
	}

	return addrs
}

func (np *NbrPeers) GetNeighborHeights() map[uint64]uint64 {
	np.RLock()
	defer np.RUnlock()

	hm := make(map[uint64]uint64)
	for _, n := range np.List {
		if n.GetSyncState() == common.ESTABLISH {
			hm[n.GetID()] = n.GetHeight()
		}
	}
	return hm
}

func (np *NbrPeers) GetNeighbors() []*Peer {
	np.RLock()
	defer np.RUnlock()
	peers := []*Peer{}
	for _, n := range np.List {
		if n.GetSyncState() == common.ESTABLISH {
			node := n
			peers = append(peers, node)
		}
	}
	return peers
}

func (np *NbrPeers) GetNbrNodeCnt() uint32 {
	np.RLock()
	defer np.RUnlock()
	var count uint32
	for _, n := range np.List {
		if n.GetSyncState() == common.ESTABLISH {
			count++
		}
	}
	return count
}
