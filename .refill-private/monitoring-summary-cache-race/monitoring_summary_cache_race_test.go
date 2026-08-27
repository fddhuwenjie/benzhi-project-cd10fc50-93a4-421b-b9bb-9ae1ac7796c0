package monitoring_summary_cache_race_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"timber-pest-remediation-ledger/internal/cases"
	"timber-pest-remediation-ledger/internal/domain"
	"timber-pest-remediation-ledger/internal/store"
)

func TestConcurrentMonitoringSummariesIsolateCache(t *testing.T) {
	ctx := context.Background()
	repository, err := store.Open(ctx, filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	service := cases.NewService(repository)
	defer service.Close()

	prepareCase(t, service, "case-a", 48)
	prepareCase(t, service, "case-b", 48)

	const readers = 16
	start := make(chan struct{})
	errors := make(chan error, readers)
	var ready sync.WaitGroup
	var done sync.WaitGroup
	ready.Add(readers)
	done.Add(readers)
	for reader := 0; reader < readers; reader++ {
		caseID := "case-a"
		if reader%2 == 1 {
			caseID = "case-b"
		}
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			for iteration := 0; iteration < 2; iteration++ {
				summary, queryErr := service.MonitoringSummary(ctx, caseID)
				if queryErr != nil {
					errors <- queryErr
					return
				}
				if len(summary.Zones) != 48 {
					errors <- fmt.Errorf("%s 返回 %d 个分区", caseID, len(summary.Zones))
					return
				}
				for _, zone := range summary.Zones {
					if !strings.HasPrefix(zone.ZoneID, caseID+"-zone-") {
						errors <- fmt.Errorf("%s 的摘要混入分区 %s", caseID, zone.ZoneID)
						return
					}
				}
			}
		}()
	}
	ready.Wait()
	close(start)
	done.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func prepareCase(t *testing.T, service *cases.Service, caseID string, zoneCount int) {
	t.Helper()
	ctx := context.Background()
	components := make([]domain.TimberComponent, 0, zoneCount)
	zones := make([]domain.ZonePlanInput, 0, zoneCount)
	for index := 0; index < zoneCount; index++ {
		componentID := fmt.Sprintf("%s-component-%02d", caseID, index)
		components = append(components, domain.TimberComponent{
			ComponentID: componentID, LocationCode: fmt.Sprintf("L-%02d", index),
			ComponentType: "梁", HeritageGrade: "I", PestClue: "蛀孔",
			ActivityScore: 3, DamageExtentPercent: 12, MoisturePercent: 16,
			EvidenceDigest: fmt.Sprintf("evidence-%02d", index),
		})
		zones = append(zones, domain.ZonePlanInput{
			ZoneID: fmt.Sprintf("%s-zone-%02d", caseID, index), ComponentIDs: []string{componentID},
			Method: "局部熏蒸", ApprovedParameters: map[string]float64{"dose": 1},
			ProtectionConstraints: []string{"遮护彩画"}, ResponsibleID: "field-lead",
			AcceptanceThresholds: domain.AcceptanceThresholds{MaxActivityCount: 0, MaxAcousticScore: 0.2, MinObservations: 1},
		})
	}
	created, err := service.Create(ctx, cases.CreateCommand{
		RequestID: caseID + "-create", ActorID: "survey", CaseID: caseID,
		SiteName: "古建", BuildingZone: "正殿", SurveyLeadID: "survey", Components: components,
	})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := service.FreezeBaseline(ctx, caseID, cases.CommandMeta{RequestID: caseID + "-freeze", ActorID: "survey", ExpectedRevision: created.Revision})
	if err != nil {
		t.Fatal(err)
	}
	assessed, err := service.AssessRisk(ctx, caseID, cases.CommandMeta{RequestID: caseID + "-risk", ActorID: "engineer", ExpectedRevision: frozen.Revision})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.SubmitPlan(ctx, caseID, cases.PlanCommand{
		CommandMeta: cases.CommandMeta{RequestID: caseID + "-plan", ActorID: "planner", ExpectedRevision: assessed.Revision},
		Zones:       zones,
	})
	if err != nil {
		t.Fatal(err)
	}
}
