package sledger

type sledger struct {
	Sledger []struct {
		Forward  string
		Backward string
	}
}

type rollback struct {
	index      int
	dbBackward string
}
