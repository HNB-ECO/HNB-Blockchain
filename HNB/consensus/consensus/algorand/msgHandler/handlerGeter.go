package msgHandler

func (tdm *TDMMsgHandler) AllRoutineExitWg() {
	tdm.allRoutineExitWg.Wait()
}
