package pipeline

type PipelineBaseHandler struct {
	Next PipelineHandler
}

func (h *PipelineBaseHandler) SetNext(next PipelineHandler) {
	h.Next = next
}
