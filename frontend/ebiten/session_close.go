package ebiten

// finishSession 讓關閉中的消費端仍可透過 Update 取得 GPU 服務（025 §11）。
// 僅在機器已停止輸入後呼叫；正常 Close 在 Cancel 之前完成，且只執行一次。
func (g *Game) finishSession() bool {
	if g.sessionClosing == nil {
		g.sessionClosing = make(chan struct{})
		var finish func()
		if g.sess != nil {
			finish = g.sess.Close
			g.sess.Close = nil
		}
		done := g.sessionClosing
		go func() {
			if finish != nil {
				finish()
			}
			close(done)
		}()
	}
	select {
	case <-g.sessionClosing:
		return true
	default:
		return false
	}
}
