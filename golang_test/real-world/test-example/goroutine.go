package test_example

import (
	"errors"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

var log = logrus.WithField("prefix", "service")

type Service struct {
	connected	bool
	isRunning   bool
	wg 			*sync.WaitGroup
}

func NewService() *Service {
	return &Service{
		wg: new(sync.WaitGroup),
	}
}

func (s *Service) Start() error {
	s.wg.Add(1)

	go func() error {
		s.isRunning = true
		time.Sleep(5 * time.Second)
		if s.connected == false {
			s.wg.Done()
			return errors.New("cannot start service!")
		}
		return nil
	}()

	s.wg.Wait()
	return nil
}
