package log

import (
	golog "log"
)

type ConsoleLogAdapter struct {
}

func NewConsoleLogAdapter() LogAdapter {
	return new(ConsoleLogAdapter)
}

func (cla *ConsoleLogAdapter) Debugf(lc *LogContext, message *string) error {
	golog.Println(*message)

	return nil
}

func (cla *ConsoleLogAdapter) Infof(lc *LogContext, message *string) error {
	golog.Println(*message)

	return nil
}

func (cla *ConsoleLogAdapter) Warningf(lc *LogContext, message *string) error {
	golog.Println(*message)

	return nil
}

func (cla *ConsoleLogAdapter) Errorf(lc *LogContext, message *string) error {
	golog.Println(*message)

	return nil
}
