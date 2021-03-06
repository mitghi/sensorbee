package core

import (
	"fmt"
	"gopkg.in/sensorbee/sensorbee.v0/data"
)

type defaultBoxNode struct {
	*defaultNode
	config *BoxConfig
	srcs   *dataSources
	box    Box
	dsts   *dataDestinations

	gracefulStopEnabled bool
	stopOnDisconnectDir ConnDir
	runErr              error
}

func (db *defaultBoxNode) Type() NodeType {
	return NTBox
}

func (db *defaultBoxNode) Box() Box {
	return db.box
}

func (db *defaultBoxNode) Input(refname string, config *BoxInputConfig) error {
	s, err := db.topology.dataSource(refname)
	if err != nil {
		return err
	}

	if config == nil {
		config = defaultBoxInputConfig
	}
	if err := config.Validate(); err != nil {
		return err
	}

	if err := checkBoxInputName(db.box, db.name, config.inputName()); err != nil {
		return err
	}

	recv, send := newPipe(config.inputName(), config.capacity())
	send.dropMode = config.DropMode
	if err := s.destinations().add(db.name, send); err != nil {
		return err
	}
	if err := db.srcs.add(s.Name(), recv); err != nil {
		s.destinations().remove(db.name)
		return err
	}
	return nil
}

func (db *defaultBoxNode) run() (runErr error) {
	if err := db.checkAndPrepareForRunning("box"); err != nil {
		return err
	}

	defer func() {
		defer func() {
			if e := recover(); e != nil {
				if db.runErr == nil {
					db.runErr = fmt.Errorf("the box couldn't be terminated due to panic: %v", e)
				} else {
					db.topology.ctx.ErrLog(fmt.Errorf("%v", e)).WithFields(nodeLogFields(NTBox, db.name)).
						Error("Cannot terminate the box due to panic")
				}
			}
			runErr = db.runErr
			db.dsts.Close(db.topology.ctx)
			db.state.Set(TSStopped)
		}()
		if sb, ok := db.box.(StatefulBox); ok {
			if err := sb.Terminate(db.topology.ctx); err != nil {
				if db.runErr == nil {
					db.runErr = err
				} else {
					db.topology.ctx.ErrLog(err).WithFields(nodeLogFields(NTBox, db.name)).
						Error("Cannot terminate the box")
				}
			}
		}
	}()
	db.state.Set(TSRunning)
	w := newBoxWriterAdapter(db.box, db.name, db.dsts)
	db.runErr = db.srcs.pour(db.topology.ctx, w, 1) // TODO: make parallelism configurable
	return
}

func (db *defaultBoxNode) Stop() error {
	db.stop()
	return nil
}

func (db *defaultBoxNode) EnableGracefulStop() {
	db.stateMutex.Lock()
	db.gracefulStopEnabled = true
	db.stateMutex.Unlock()
	db.srcs.enableGracefulStop()
}

func (db *defaultBoxNode) StopOnDisconnect(dir ConnDir) {
	db.stateMutex.Lock()
	db.stopOnDisconnectDir |= dir
	dir = db.stopOnDisconnectDir
	db.stateMutex.Unlock()

	if dir&Inbound != 0 {
		db.srcs.stopOnDisconnect()
	}
	if dir&Outbound != 0 {
		if db.dsts.len() == 0 {
			db.stop()
		}
	}
}

func (db *defaultBoxNode) stop() {
	if stopped, err := db.checkAndPrepareForStopping("box"); stopped || err != nil {
		return
	}

	db.state.Set(TSStopping)
	db.srcs.stop(db.topology.ctx) // waits until all tuples get processed.
	db.state.Wait(TSStopped)
}

func (db *defaultBoxNode) Status() data.Map {
	db.stateMutex.Lock()
	st := db.state.getWithoutLock()
	gstop := db.gracefulStopEnabled
	connDir := db.stopOnDisconnectDir
	removeOnStop := db.config.RemoveOnStop
	db.stateMutex.Unlock()

	m := data.Map{
		"state":        data.String(st.String()),
		"input_stats":  db.srcs.status(),
		"output_stats": db.dsts.status(),
		"behaviors": data.Map{
			"stop_on_inbound_disconnect":  data.Bool((connDir & Inbound) != 0),
			"stop_on_outbound_disconnect": data.Bool((connDir & Outbound) != 0),
			"graceful_stop":               data.Bool(gstop),
			"remove_on_stop":              data.Bool(removeOnStop),
		},
	}
	if st == TSStopped && db.runErr != nil {
		m["error"] = data.String(db.runErr.Error())
	}
	if b, ok := db.box.(Statuser); ok {
		m["box"] = b.Status()
	}
	return m
}

func (db *defaultBoxNode) destinations() *dataDestinations {
	return db.dsts
}

func (db *defaultBoxNode) dstCallback(e ddEvent) {
	switch e {
	case ddeDisconnect:
		db.stateMutex.Lock()
		shouldStop := (db.stopOnDisconnectDir & Outbound) != 0
		db.stateMutex.Unlock()

		if shouldStop {
			db.stop()
		}
	}
}

func (db *defaultBoxNode) RemoveOnStop() {
	db.stateMutex.Lock()
	db.config.RemoveOnStop = true
	st := db.state.getWithoutLock()
	db.stateMutex.Unlock()

	if st == TSStopped {
		db.topology.Remove(db.name)
	}
}
