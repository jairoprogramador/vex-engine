package command

type BaseExecutable struct{}

func (b *BaseExecutable) Run(
	executionContext *ExecutionContext,
	before func() error,
	exec func() error,
	after func() error,
) error {

	if before != nil {
		if err := before(); err != nil {
			return err
		}
	}

	if err := exec(); err != nil {
		return err // 🔥 fail-fast
	}

	if after != nil {
		if err := after(); err != nil {
			return err
		}
	}

	return nil
}
