package main

import (
	"container/list"
	"errors"
	"sync"
)

type NetworkInfo[N any] struct {
	// Add necessary fields
}

type EpochState[C any, N any] struct {
	// Add necessary fields
}

type Params struct {
	maxFutureEpochs        uint64
	subsetHandlingStrategy string
	encryptionSchedule     EncryptionSchedule
}

type EncryptionSchedule int

const (
	Always EncryptionSchedule = iota
	Never
	EveryNthEpoch
	TickTock
)

type HoneyBadger[C any, N any] struct {
	netinfo    *NetworkInfo[N]
	sessionID  uint64
	epoch      uint64
	hasInput   bool
	epochs     map[uint64]*EpochState[C, N]
	params     Params
}

type Step[C any, N any] struct {
	output    []Batch[C, N]
	faultLog  []Fault
}

type Batch[C any, N any] struct {
	// Add necessary fields
}

type Message[N any] struct {
	epoch   uint64
	content string
}

type Fault struct {
	// Add necessary fields
}

type Result[T any] struct {
	// Add necessary fields
}

func NewHoneyBadger[C any, N any](netinfo *NetworkInfo[N], sessionID uint64, params Params) *HoneyBadger[C, N] {
	return &HoneyBadger[C, N]{
		netinfo:   netinfo,
		sessionID: sessionID,
		epoch:     0,
		hasInput:  false,
		epochs:    make(map[uint64]*EpochState[C, N]),
		params:    params,
	}
}

func (hb *HoneyBadger[C, N]) HandleInput(input C, rng *rand.Rand) (Step[C, N], error) {
	return hb.propose(&input, rng)
}

func (hb *HoneyBadger[C, N]) HandleMessage(senderID N, message Message[N], rng *rand.Rand) (Step[C, N], error) {
	return hb.handleMessage(senderID, message)
}

func (hb *HoneyBadger[C, N]) Terminated() bool {
	return false
}

func (hb *HoneyBadger[C, N]) OurID() N {
	return hb.netinfo.ourID()
}

func (hb *HoneyBadger[C, N]) propose(proposal *C, rng *rand.Rand) (Step[C, N], error) {
	if !hb.netinfo.isValidator() {
		return Step[C, N]{}, nil
	}
	hb.hasInput = true
	step, err := hb.epochStateMut(hb.epoch).propose(proposal, rng)
	if err != nil {
		return Step[C, N]{}, err
	}
	return step.join(hb.tryOutputBatches())
}

func (hb *HoneyBadger[C, N]) handleMessage(senderID N, message Message[N]) (Step[C, N], error) {
	if !hb.netinfo.isNodeValidator(senderID) {
		return Step[C, N]{}, errors.New("unknown sender")
	}
	if message.epoch > hb.epoch+hb.params.maxFutureEpochs {
		return Step[C, N]{}, errors.New("unexpected HB message epoch")
	} else if message.epoch < hb.epoch {
		return Step[C, N]{}, nil
	} else {
		step, err := hb.epochStateMut(message.epoch).handleMessageContent(senderID, message.content)
		if err != nil {
			return Step[C, N]{}, err
		}
		return step.join(hb.tryOutputBatches())
	}
}

func (hb *HoneyBadger[C, N]) HasInput() bool {
	return !hb.netinfo.isValidator() || hb.hasInput
}

func (hb *HoneyBadger[C, N]) GetEncryptionSchedule() EncryptionSchedule {
	return hb.params.encryptionSchedule
}

func (hb *HoneyBadger[C, N]) NextEpoch() uint64 {
	return hb.epoch
}

func (hb *HoneyBadger[C, N]) Netinfo() *NetworkInfo[N] {
	return hb.netinfo
}

func (hb *HoneyBadger[C, N]) SkipToEpoch(epoch uint64) {
	for hb.epoch < epoch {
		hb.updateEpoch()
	}
}

func (hb *HoneyBadger[C, N]) ReceivedProposals() int {
	if epochState, ok := hb.epochs[hb.epoch]; ok {
		return epochState.receivedProposals()
	}
	return 0
}

func (hb *HoneyBadger[C, N]) updateEpoch() {
	delete(hb.epochs, hb.epoch)
	hb.epoch++
	hb.hasInput = false
}

func (hb *HoneyBadger[C, N]) tryOutputBatches() (Step[C, N], error) {
	var step Step[C, N]
	for {
		if batch, faultLog := hb.epochs[hb.epoch].tryOutputBatch(); batch != nil {
			step.output = append(step.output, *batch)
			step.faultLog = append(step.faultLog, faultLog...)
			hb.updateEpoch()
		} else {
			break
		}
	}
	return step, nil
}

func (hb *HoneyBadger[C, N]) epochStateMut(epoch uint64) (*EpochState[C, N], error) {
	if epochState, ok := hb.epochs[epoch]; ok {
		return epochState, nil
	}
	newEpochState, err := NewEpochState(hb.netinfo, hb.sessionID, epoch, hb.params.subsetHandlingStrategy, hb.params.encryptionSchedule.UseOnEpoch(epoch))
	if err != nil {
		return nil, err
	}
	hb.epochs[epoch] = newEpochState
	return newEpochState, nil
}

func (es EncryptionSchedule) UseOnEpoch(epoch uint64) bool {
	switch es {
	case Always:
		return true
	case Never:
		return false
	case EveryNthEpoch:
		return (epoch % uint64(es)) == 0
	case TickTock:
		return (epoch % uint64(es)) <= uint64(es)
	default:
		return false
	}
}


