package msgHandler

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/op/go-logging"
	"sync"
)

type CheckFork struct {
	sync.RWMutex
	fork   map[uint64]map[uint64][]*PeerHash
	Logger *logging.Logger
}

type PeerHash struct {
	peerid  uint64
	preHash []byte
}

func NewCheckFork() *CheckFork {
	return &CheckFork{
		fork: make(map[uint64]map[uint64][]*PeerHash),
	}
}

func (cfh *CheckFork) isExists(Bgid uint64) bool {
	cfh.RWMutex.RLock()
	defer cfh.RWMutex.RUnlock()
	_, ok := cfh.fork[Bgid]
	return ok
}

func (cfh *CheckFork) initFork(Bgid uint64) {
	cfh.RWMutex.Lock()
	fork := make(map[uint64][]*PeerHash)
	cfh.fork[Bgid] = fork
	cfh.RWMutex.Unlock()
}

func (cfh *CheckFork) isDuplicate(Bgid uint64, height uint64, peerid uint64) (bool, error) {
	cfh.RWMutex.RLock()
	defer cfh.RWMutex.RUnlock()

	fork := cfh.fork[Bgid]
	ph, ok := fork[height]
	if !ok {
		return false, nil
	}
	for _, p := range ph {
		if p == nil {
			return false, fmt.Errorf("Bgid %d, height %d store nil", Bgid, height)
		}
		if p.peerid == peerid {
			return true, nil
		}
	}
	return false, nil
}

func (cfh *CheckFork) addFork(Bgid uint64, height uint64, peerHash *PeerHash) error {
	if peerHash == nil {
		return fmt.Errorf("peerHash is nil ")
	}
	cfh.Lock()
	fork := cfh.fork[Bgid]
	ph := fork[height]
	cfh.Logger.Infof("addFork before Bgid %d, height %d len %d ", Bgid, height, len(ph))
	ph = append(ph, peerHash)
	fork[height] = ph
	cfh.fork[Bgid] = fork
	cfh.Unlock()
	return nil
}

func (cfh *CheckFork) clean(Bgid uint64) {
	cfh.RWMutex.Lock()
	for bgid, _ := range cfh.fork {
		if bgid <= Bgid {
			delete(cfh.fork, bgid)
		}
	}
	cfh.RWMutex.Unlock()
}

func (cfh *CheckFork) isCountToTarget(Bgid uint64, height uint64, preHash []byte, targetCount int) (bool, []uint64) {
	cfh.RWMutex.RLock()
	defer cfh.RWMutex.RUnlock()
	fork := cfh.fork[Bgid]
	//if len(fork[height]) < targetCount {
	//	return false
	//}
	//count := 0
	peers := make([]uint64, 0)
	cfh.Logger.Infof("Bgid %d, height %d preHash len %d ",
		Bgid, height, len(fork[height]))
	for index, ph := range fork[height] {
		if ph == nil {
			cfh.Logger.Warningf("Bgid %d, height %d store nil", Bgid, height)
			continue
		}
		cfh.Logger.Infof("index %d Bgid %d, height %d hash store %s new %s", index, Bgid, height,
			hex.EncodeToString(ph.preHash), hex.EncodeToString(preHash))
		if bytes.Equal(preHash, ph.preHash) {
			//count++
			peers = append(peers, ph.peerid)
		}
	}
	if len(peers) < targetCount {
		cfh.Logger.Infof("Bgid %d, height %d preHash %s count %d target %d",
			Bgid, height, hex.EncodeToString(preHash), len(peers), targetCount)
		return false, nil
	}
	return true, peers
}

