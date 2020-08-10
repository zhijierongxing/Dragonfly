package preheat

import (
	"errors"
	"github.com/sirupsen/logrus"
	"time"

	"github.com/dragonflyoss/Dragonfly/apis/types"
	"github.com/dragonflyoss/Dragonfly/supernode/daemon/mgr"
)

func init() {
	RegisterPreheater("file", &FilePreheat{BasePreheater:new(BasePreheater)})
	logrus.StandardLogger().SetLevel(logrus.DebugLevel)
}

type FilePreheat struct {
	*BasePreheater
}

func (p *FilePreheat) Type() string {
	return "file"
}

/**
 * Create a worker to preheat the task.
 */
func (p *FilePreheat) NewWorker(task *mgr.PreheatTask , service *PreheatService) IWorker {
	worker := &FileWorker{BaseWorker: newBaseWorker(task, p, service)}
	worker.worker = worker
	p.addWorker(task.ID, worker)
	return worker
}

type FileWorker struct {
	*BaseWorker
	progress *PreheatProgress
}

func (w *FileWorker) preRun() bool {
	w.Task.Status = types.PreheatStatusRUNNING
	w.PreheatService.Update(w.Task.ID, w.Task)
	var err error
	w.progress, err = w.PreheatService.ExecutePreheat(w.Task)
	if err != nil {
		w.failed(err.Error())
		return false
	}
	return true
}

func (w *FileWorker) afterRun() {
	if w.progress != nil {
		w.progress.cmd.Process.Kill()
	}
	w.BaseWorker.afterRun()
}

func (w *FileWorker) query() chan error {
	result := make(chan error, 1)
	go func(){
		time.Sleep(time.Second*2)
		for w.isRunning() {
			if w.Task.FinishTime > 0 {
				w.Preheater.Cancel(w.Task.ID)
				return
			}
			if w.progress == nil {
				w.succeed()
				return
			}
			status := w.progress.cmd.ProcessState
			if status != nil && status.Exited() {
				if !status.Success() {
					errMsg := "dfget failed:" + status.String()
					w.failed(errMsg)
					w.Preheater.Cancel(w.Task.ID)
					result <- errors.New(errMsg)
					return
				} else {
					w.succeed()
					w.Preheater.Cancel(w.Task.ID)
					result <- nil
					return
				}
			}

			time.Sleep(time.Second*10)
		}
	}()
	return result
}

