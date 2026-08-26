package cases

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"timber-pest-remediation-ledger/internal/archive"
	"timber-pest-remediation-ledger/internal/domain"
	"timber-pest-remediation-ledger/internal/store"
)

type Service struct {
	store       *store.Store
	coordinator *Coordinator
	clock       Clock
}

func NewService(repository *store.Store) *Service {
	return &Service{store: repository, coordinator: NewCoordinator(32, 2048, 5*time.Minute), clock: realClock{}}
}
func (s *Service) Close() { s.coordinator.Close() }

func (s *Service) Create(ctx context.Context, command CreateCommand) (*domain.RemediationCase, error) {
	if err := validateIdentity(command.RequestID, command.ActorID); err != nil {
		return nil, err
	}
	if command.ActorID != command.SurveyLeadID {
		return nil, domain.Gate("survey_identity", "建案 actor_id 必须与 survey_lead_id 一致")
	}
	value, err := s.coordinator.Do(ctx, command.CaseID, func() (any, error) {
		c, event, err := domain.NewCase(domain.CreateCaseInput{CaseID: command.CaseID, SiteName: command.SiteName, BuildingZone: command.BuildingZone, SurveyLeadID: command.SurveyLeadID, Components: command.Components}, s.clock.Now())
		if err != nil {
			return nil, err
		}
		fingerprint, err := fingerprint("create", command)
		if err != nil {
			return nil, err
		}
		result, err := s.store.Create(ctx, c, event, command.RequestID, fingerprint)
		if err != nil {
			return nil, err
		}
		return result.Case, nil
	})
	if err != nil {
		return nil, err
	}
	return value.(*domain.RemediationCase), nil
}

func (s *Service) FreezeBaseline(ctx context.Context, caseID string, meta CommandMeta) (*domain.RemediationCase, error) {
	return s.commit(ctx, caseID, "freeze_baseline", meta, meta, func(c *domain.RemediationCase) (domain.EventDraft, error) {
		return c.FreezeBaseline(meta.ActorID, s.clock.Now())
	}, nil)
}

func (s *Service) ReviseBaselineComponents(ctx context.Context, caseID string, command ComponentRevisionCommand) (*domain.RemediationCase, error) {
	return s.commit(ctx, caseID, "revise_baseline_components", command.CommandMeta, command, func(c *domain.RemediationCase) (domain.EventDraft, error) {
		return c.ReviseBaselineComponents(command.ActorID, command.Operations)
	}, nil)
}

func (s *Service) AssessRisk(ctx context.Context, caseID string, meta CommandMeta) (*domain.RemediationCase, error) {
	return s.commit(ctx, caseID, "assess_risk", meta, meta, func(c *domain.RemediationCase) (domain.EventDraft, error) {
		return c.AssessRisk(meta.ActorID, s.clock.Now())
	}, nil)
}

func (s *Service) SubmitPlan(ctx context.Context, caseID string, command PlanCommand) (*domain.RemediationCase, error) {
	return s.commit(ctx, caseID, "submit_plan", command.CommandMeta, command, func(c *domain.RemediationCase) (domain.EventDraft, error) {
		return c.SubmitPlan(command.ActorID, command.Zones, s.clock.Now())
	}, nil)
}

func (s *Service) ApprovePlan(ctx context.Context, caseID string, meta CommandMeta) (*domain.RemediationCase, error) {
	return s.commit(ctx, caseID, "approve_plan", meta, meta, func(c *domain.RemediationCase) (domain.EventDraft, error) {
		return c.ApprovePlan(meta.ActorID, s.clock.Now())
	}, nil)
}

func (s *Service) ReturnPlan(ctx context.Context, caseID string, command PlanReturnCommand) (*domain.RemediationCase, error) {
	return s.commit(ctx, caseID, "return_plan", command.CommandMeta, command, func(c *domain.RemediationCase) (domain.EventDraft, error) {
		return c.ReturnPlan(command.ActorID, command.Reason, command.ZoneIDs, s.clock.Now())
	}, nil)
}

func (s *Service) RecordExecution(ctx context.Context, caseID, zoneID string, command ExecutionCommand) (*domain.RemediationCase, error) {
	return s.commit(ctx, caseID, "record_execution:"+zoneID, command.CommandMeta, command, func(c *domain.RemediationCase) (domain.EventDraft, error) {
		return c.RecordExecution(zoneID, command.ActorID, command.Execution, s.clock.Now())
	}, nil)
}

func (s *Service) RecordExecutionBatch(ctx context.Context, caseID string, command ExecutionBatchCommand) (*domain.RemediationCase, error) {
	return s.commit(ctx, caseID, "record_execution_batch", command.CommandMeta, command, func(c *domain.RemediationCase) (domain.EventDraft, error) {
		return c.RecordExecutionBatch(command.ActorID, command.Executions, s.clock.Now())
	}, nil)
}

func (s *Service) DecideRemediation(ctx context.Context, caseID, zoneID string, command RemediationCommand) (*domain.RemediationCase, error) {
	return s.commit(ctx, caseID, "decide_remediation:"+zoneID, command.CommandMeta, command, func(c *domain.RemediationCase) (domain.EventDraft, error) {
		return c.DecideRemediation(zoneID, command.ActorID, command.Decision)
	}, nil)
}

func (s *Service) AddObservation(ctx context.Context, caseID, zoneID string, command ObservationCommand) (*domain.RemediationCase, error) {
	return s.commit(ctx, caseID, "add_observation:"+zoneID, command.CommandMeta, command, func(c *domain.RemediationCase) (domain.EventDraft, error) {
		return c.AddObservation(zoneID, command.ActorID, command.Observation, s.clock.Now())
	}, nil)
}

func (s *Service) EvaluateZone(ctx context.Context, caseID, zoneID string, meta CommandMeta) (*domain.RemediationCase, error) {
	return s.commit(ctx, caseID, "evaluate_zone:"+zoneID, meta, meta, func(c *domain.RemediationCase) (domain.EventDraft, error) {
		return c.EvaluateZone(zoneID, meta.ActorID)
	}, nil)
}

func (s *Service) Review(ctx context.Context, caseID string, command ReviewCommand) (*domain.RemediationCase, error) {
	var builder store.ArchiveBuilder
	if strings.EqualFold(strings.TrimSpace(command.Decision), "pass") {
		builder = archive.Build
	}
	return s.commit(ctx, caseID, "review", command.CommandMeta, command, func(c *domain.RemediationCase) (domain.EventDraft, error) {
		return c.ReviewCase(command.ActorID, command.Decision, command.Findings, command.EvidenceComplete, s.clock.Now())
	}, builder)
}

func (s *Service) commit(ctx context.Context, caseID, operation string, meta CommandMeta, body any, mutation store.Mutation, builder store.ArchiveBuilder) (*domain.RemediationCase, error) {
	if caseID == "" {
		return nil, domain.Invalid("case_id_required", "case_id 为必填")
	}
	if err := validateIdentity(meta.RequestID, meta.ActorID); err != nil {
		return nil, err
	}
	if meta.ExpectedRevision < 1 {
		return nil, domain.Invalid("expected_revision_required", "expected_revision 必须大于 0")
	}
	fp, err := fingerprint(operation, body)
	if err != nil {
		return nil, err
	}
	value, err := s.coordinator.Do(ctx, caseID, func() (any, error) {
		return s.store.Commit(ctx, caseID, meta.ExpectedRevision, meta.RequestID, fp, mutation, builder)
	})
	if err != nil {
		return nil, err
	}
	return value.(*store.CommitResult).Case, nil
}

func validateIdentity(requestID, actorID string) error {
	requestID = strings.TrimSpace(requestID)
	actorID = strings.TrimSpace(actorID)
	if requestID == "" || len(requestID) > 128 {
		return domain.Invalid("request_id_invalid", "request_id 必填且最长 128 字符")
	}
	if actorID == "" || len(actorID) > 128 {
		return domain.Invalid("actor_id_invalid", "actor_id 必填且最长 128 字符")
	}
	return nil
}

func fingerprint(operation string, body any) (string, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("编码请求指纹: %w", err)
	}
	h := sha256.New()
	h.Write([]byte(operation))
	h.Write([]byte{0})
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil)), nil
}
