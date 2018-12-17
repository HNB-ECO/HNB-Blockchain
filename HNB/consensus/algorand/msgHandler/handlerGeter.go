package msgHandler

//func (tdm *TDMMsgHandler) GetAddr() cmn.HexBytes {
//	return tdm.digestAddr
//}

//func (tdm *TDMMsgHandler) GetAllRoutineExitWg() sync.WaitGroup {
//	return tdm.allRoutineExitWg
//}

func (tdm *TDMMsgHandler) AllRoutineExitWg() {
	tdm.allRoutineExitWg.Wait()
}
