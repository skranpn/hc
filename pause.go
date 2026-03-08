package hc

import "sync"

type PauseController struct {
	mu     sync.Mutex
	cond   *sync.Cond
	paused bool
}

func NewPauseController() *PauseController {
	c := &PauseController{}
	c.cond = sync.NewCond(&c.mu)
	return c
}

func (c *PauseController) WaitIfPaused() {
	c.mu.Lock()
	for c.paused {
		c.cond.Wait() // 再開のシグナルが来るまでブロック
	}
	c.mu.Unlock()
}

func (c *PauseController) Toggle() {
	c.mu.Lock()
	c.paused = !c.paused
	if !c.paused {
		c.cond.Broadcast() // 全ての待機中の処理を再開
	}
	c.mu.Unlock()
}
