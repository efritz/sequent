package sequent

func blockOnSignal(halt <-chan struct{}, ready <-chan struct{}) bool {
	select {
	case <-halt:
		return false

	case _, ok := <-ready:
		return ok
	}
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true

	default:
		return false
	}
}
