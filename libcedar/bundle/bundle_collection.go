package bundle

import (
	"errors"
	"sync"
)

var ErrDuplicatedBundle = errors.New("duplicated bundle")

type BundleCollection struct {
	sync.RWMutex
	data map[uint32]*FiberBundle
	main *FiberBundle
}

func NewBundleCollection() *BundleCollection {
	ret := new(BundleCollection)
	ret.data = make(map[uint32]*FiberBundle)
	ret.main = nil
	return ret
}

func (bc *BundleCollection) AddBundle(bd *FiberBundle) error {
	bc.Lock()
	defer bc.Unlock()

	id := bd.id
	_, ok := bc.data[id]
	if ok {
		return ErrDuplicatedBundle
	}
	bc.data[id] = bd
	bc.main = bd

	return nil
}

func (bc *BundleCollection) HasID(id uint32) bool {
	bc.RLock()
	defer bc.RUnlock()
	_, ok := bc.data[id]
	return ok
}

func (bc *BundleCollection) GetBundle(id uint32) *FiberBundle {
	bc.RLock()
	defer bc.RUnlock()
	if id == 0 {
		return bc.main
	}
	v, ok := bc.data[id]
	if ok {
		return v
	}
	return nil
}

func (bc *BundleCollection) HasMain() bool {
	bc.RLock()
	defer bc.RUnlock()
	return bc.main != nil
}

func (bc *BundleCollection) GetMain() *FiberBundle {
	bc.RLock()
	defer bc.RUnlock()
	if bc.main != nil {
		return bc.main
	}
	return nil
}
