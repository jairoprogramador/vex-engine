package command

type CommandBaseHandler struct {
	Next CommandHandler
}

func (h *CommandBaseHandler) SetNext(next CommandHandler) {
	h.Next = next
}
