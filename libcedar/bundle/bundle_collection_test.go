package bundle

import "testing"

func TestBundleCollectionBasic(t *testing.T) {
	bdc := NewBundleCollection()

	if bdc.HasID(5) || bdc.HasMain() {
		panic("bundle collection should be empty")
	}

	bd := NewFiberBundle(50, "server", &HandshakeResult{5, 0, 0, nil})

	err := bdc.AddBundle(bd)
	if err != nil {
		panic("adding bundle failed")
	}
	if !bdc.HasID(5) {
		panic("cannot find by id")
	}
	if !bdc.HasMain() {
		panic("cannot find main")
	}

	if bdc.GetBundle(5) != bdc.GetMain() {
		panic("getting bundle error")
	}
}
