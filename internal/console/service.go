package console

import (
	"context"
	"sync"

	"aurora/internal/audit_logging"
)

const defaultAuditSubscriptionBuffer = 128

type Service struct {
	broadcaster *Broadcaster
	logger      auditlog.LoggerInterface
	stop        context.CancelFunc
	once        sync.Once
}

func NewService(logger auditlog.LoggerInterface, recentBufferSize int) *Service {
	svc := &Service{
		broadcaster: NewBroadcaster(recentBufferSize),
		logger:      logger,
	}
	if logger != nil {
		ctx, cancel := context.WithCancel(context.Background())
		svc.stop = cancel
		go svc.forwardAuditEvents(ctx)
	}
	return svc
}

func (s *Service) Subscribe(bufferSize int) *Subscriber {
	if s == nil || s.broadcaster == nil {
		closed := make(chan Event)
		close(closed)
		return &Subscriber{ID: "noop", Events: closed}
	}
	return s.broadcaster.Subscribe(bufferSize)
}

func (s *Service) Unsubscribe(id string) {
	if s == nil || s.broadcaster == nil {
		return
	}
	s.broadcaster.Unsubscribe(id)
}

func (s *Service) Recent(limit, offset int) []Event {
	if s == nil || s.broadcaster == nil {
		return nil
	}
	return s.broadcaster.Recent(limit, offset)
}

func (s *Service) LenRecent() int {
	if s == nil || s.broadcaster == nil {
		return 0
	}
	return s.broadcaster.LenRecent()
}

func (s *Service) Close() {
	if s == nil || s.stop == nil {
		return
	}
	s.once.Do(s.stop)
}

func (s *Service) forwardAuditEvents(ctx context.Context) {
	sub := s.logger.SubscribeLive(defaultAuditSubscriptionBuffer)
	defer s.logger.UnsubscribeLive(sub.ID)
	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-sub.Entries:
			if !ok {
				return
			}
			event := FromAuditEntry(entry)
			if event.ID != "" {
				s.broadcaster.Publish(event)
			}
		}
	}
}
