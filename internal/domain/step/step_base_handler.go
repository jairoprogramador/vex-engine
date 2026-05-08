package step

type StepBaseHandler struct {
	Next StepHandler
}

func (h *StepBaseHandler) SetNext(next StepHandler) {
	h.Next = next
}
