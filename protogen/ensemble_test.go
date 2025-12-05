package protogen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/tgpiler/storage"
)

func TestEnsembleMapper_ShopEasyComparison(t *testing.T) {
	exampleDir := "../examples/shopeasy"
	protoDir := filepath.Join(exampleDir, "protos")
	procDir := filepath.Join(exampleDir, "procedures")

	t.Log("=" + strings.Repeat("=", 75))
	t.Log("ShopEasy: Original Mapper vs Ensemble Mapper Comparison")
	t.Log("=" + strings.Repeat("=", 75))

	// Parse protos
	parser := NewParser(protoDir)
	protoResult, err := parser.ParseDir(protoDir)
	if err != nil {
		t.Fatalf("Failed to parse protos: %v", err)
	}

	// Extract stored procedures
	extractor := storage.NewProcedureExtractor()
	files, _ := filepath.Glob(filepath.Join(procDir, "*.sql"))
	var allProcs []*storage.Procedure

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		procs, _ := extractor.ExtractAll(string(content))
		allProcs = append(allProcs, procs...)
	}

	t.Logf("Data: %d services, %d methods, %d procedures\n",
		len(protoResult.AllServices), len(protoResult.AllMethods), len(allProcs))

	// Run original mapper
	origMapper := storage.NewProtoToSQLMapper(protoResult, allProcs)
	origMappings := origMapper.MapAll()
	origStats := origMapper.GetStats()

	// Run ensemble mapper
	ensMapper := storage.NewEnsembleMapper(protoResult, allProcs)
	ensMappings := ensMapper.MapAll()
	ensStats := ensMapper.GetStats()

	// Summary comparison
	t.Log("\n[SUMMARY COMPARISON]")
	t.Log(strings.Repeat("-", 50))
	t.Logf("%-25s %10s %10s", "Metric", "Original", "Ensemble")
	t.Log(strings.Repeat("-", 50))
	t.Logf("%-25s %10d %10d", "Mapped methods", origStats.MappedMethods, ensStats.MappedMethods)
	t.Logf("%-25s %10d %10d", "Unmapped methods", origStats.UnmappedMethods, ensStats.UnmappedMethods)
	t.Logf("%-25s %10d %10d", "High confidence (>80%)", origStats.HighConfidence, ensStats.HighConfidence)
	t.Logf("%-25s %10d %10d", "Medium confidence", origStats.MediumConfidence, ensStats.MediumConfidence)
	t.Logf("%-25s %10d %10d", "Low confidence (<50%)", origStats.LowConfidence, ensStats.LowConfidence)

	// Track changes
	improved := 0
	degraded := 0
	newMappings := 0
	lostMappings := 0
	sameProc := 0
	diffProc := 0

	for svcName, svc := range protoResult.AllServices {
		for _, method := range svc.Methods {
			key := svcName + "." + method.Name
			origMap := origMappings[key]
			ensMap := ensMappings[key]

			if origMap == nil && ensMap != nil {
				newMappings++
			} else if origMap != nil && ensMap == nil {
				lostMappings++
			} else if origMap != nil && ensMap != nil {
				if origMap.Procedure.Name == ensMap.Procedure.Name {
					sameProc++
				} else {
					diffProc++
				}

				diff := ensMap.Confidence - origMap.Confidence
				if diff > 0.05 {
					improved++
				} else if diff < -0.05 {
					degraded++
				}
			}
		}
	}

	t.Log(strings.Repeat("-", 50))
	t.Logf("%-25s %10d", "Same procedure match", sameProc)
	t.Logf("%-25s %10d", "Different procedure", diffProc)
	t.Logf("%-25s %10d", "Confidence improved", improved)
	t.Logf("%-25s %10d", "Confidence degraded", degraded)
	t.Logf("%-25s %10d", "New mappings (ens only)", newMappings)
	t.Logf("%-25s %10d", "Lost mappings (orig only)", lostMappings)

	// Show methods with strategy agreement
	t.Log("\n[METHODS WITH MULTIPLE STRATEGY AGREEMENT]")
	multiStrategyCount := 0
	for svcName, svc := range protoResult.AllServices {
		for _, method := range svc.Methods {
			key := svcName + "." + method.Name
			if ensMap := ensMappings[key]; ensMap != nil {
				strategyCount := strings.Count(ensMap.MatchReason, ";") + 1
				if strategyCount >= 3 {
					multiStrategyCount++
					if multiStrategyCount <= 10 { // Show first 10
						t.Logf("  %s → %s (%.0f%%, %d strategies)",
							method.Name, ensMap.Procedure.Name, 
							ensMap.Confidence*100, strategyCount)
					}
				}
			}
		}
	}
	t.Logf("  ... %d methods with 3+ agreeing strategies", multiStrategyCount)

	// Show unmapped methods
	t.Log("\n[UNMAPPED METHODS]")
	unmappedCount := 0
	for svcName, svc := range protoResult.AllServices {
		for _, method := range svc.Methods {
			key := svcName + "." + method.Name
			if ensMappings[key] == nil {
				unmappedCount++
				t.Logf("  %s.%s", svcName, method.Name)
			}
		}
	}
	if unmappedCount == 0 {
		t.Log("  (none)")
	}

	// Show methods where mappers disagree on procedure
	if diffProc > 0 {
		t.Log("\n[METHODS WHERE MAPPERS CHOSE DIFFERENT PROCEDURES]")
		for svcName, svc := range protoResult.AllServices {
			for _, method := range svc.Methods {
				key := svcName + "." + method.Name
				origMap := origMappings[key]
				ensMap := ensMappings[key]

				if origMap != nil && ensMap != nil && origMap.Procedure.Name != ensMap.Procedure.Name {
					t.Logf("  %s.%s:", svcName, method.Name)
					t.Logf("    Original: %s (%.0f%%)", origMap.Procedure.Name, origMap.Confidence*100)
					t.Logf("    Ensemble: %s (%.0f%%) - %s", 
						ensMap.Procedure.Name, ensMap.Confidence*100, ensMap.MatchReason)
				}
			}
		}
	}

	// Assertions
	if ensStats.MappedMethods < origStats.MappedMethods {
		t.Errorf("Ensemble should not lose mappings: orig=%d, ens=%d", 
			origStats.MappedMethods, ensStats.MappedMethods)
	}

	if ensStats.HighConfidence < origStats.HighConfidence {
		t.Logf("Warning: Fewer high-confidence mappings: orig=%d, ens=%d",
			origStats.HighConfidence, ensStats.HighConfidence)
	}

	t.Log("\n" + strings.Repeat("=", 75))
}
