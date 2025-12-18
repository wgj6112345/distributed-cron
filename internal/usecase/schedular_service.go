package usecase

import (
	"context"
	"distributed-cron/internal/domain"
	"log"
	"time"
)

type SchedularService struct {
	leaderManager domain.LeaderElectionManager
	schedular     domain.Schedular
	jobRepo       domain.JobRepository
	nodeID        string
}

func NewSchedularService(leaderManager domain.LeaderElectionManager, schedular domain.Schedular, jobRepo domain.JobRepository, nodeID string) *SchedularService {
	return &SchedularService{
		leaderManager: leaderManager,
		schedular:     schedular,
		jobRepo:       jobRepo,
		nodeID:        nodeID,
	}
}

func (s *SchedularService) Start(ctx context.Context) error {
	log.Printf("Scheduler service for node %s starting...", s.nodeID)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Scheduler service for node %s shutting down.", s.nodeID)
			s.schedular.Stop()
			return ctx.Err()
		default:
			log.Printf("Node %s attempting to campaign for leadership...", s.nodeID)
			lostLeaderShipCh, err := s.leaderManager.Campaign(ctx)
			if err != nil {
				log.Printf("Node %s error during leadership locampaign: %v. Retrying in 5 seconds...", s.nodeID, err)
				time.Sleep(5 * time.Second)
				continue
			}

			log.Printf("Node %s successfully became the leader. Starting the scheduler.", s.nodeID)
			s.runSchedular(ctx)

			select {
			case <-lostLeaderShipCh:
				s.schedular.Stop()
			case <-ctx.Done():
				s.schedular.Stop()
				return ctx.Err()
			}
		}
	}
}

func (s *SchedularService) runSchedular(ctx context.Context) {

	jobs, err := s.jobRepo.List(ctx)
	if err != nil {
		log.Printf("Node %s error loading jobs for scheduler: %v", s.nodeID, err)
		return
	}

	for _, job := range jobs {
		if err = s.schedular.AddJob(job); err != nil {
			return
		}
	}

	go func() {
		if err := s.schedular.Start(ctx); err != nil {
		}
	}()
}
