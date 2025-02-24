package fluent

import (
	"github.com/fluent/fluent-logger-golang/fluent"
)

type Sender interface {
	Write(tag string, msg interface{}) error
}

type FluentSender struct {
	logger *fluent.Fluent
}

func New(targetURL string, targetPort int) (*FluentSender, error) {
	logger, err := fluent.New(fluent.Config{
		FluentPort:    targetPort,
		FluentHost:    targetURL,
		MarshalAsJSON: false,
	})

	if err != nil {
		return nil, err
	}

	return &FluentSender{
		logger: logger,
	}, nil
}

func (s *FluentSender) Write(tag string, msg interface{}) error {
	return s.logger.Post(tag, msg)
}
