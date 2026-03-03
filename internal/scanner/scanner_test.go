package scanner

import (
	"testing"
	"github.com/dablon/kubesentinel/internal/config"
	"github.com/dablon/kubesentinel/pkg/models"
)

func TestNew(t *testing.T) {
	cfg := config.Default()
	s := New(cfg)
	if s == nil {
		t.Fatal("New() returned nil")
	}
}

func TestScanAll(t *testing.T) {
	cfg := config.Default()
	s := New(cfg)
	
	result, err := s.Scan("all", "all")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
}

func TestScanNamespace(t *testing.T) {
	cfg := config.Default()
	s := New(cfg)
	
	result, err := s.Scan("production", "all")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(result.Issues) == 0 {
		t.Error("Expected issues in production")
	}
}

func TestScanSeverityFilter(t *testing.T) {
	cfg := config.Default()
	s := New(cfg)
	
	result, _ := s.Scan("all", "critical")
	for _, issue := range result.Issues {
		if issue.Severity != "critical" {
			t.Errorf("Expected critical, got %s", issue.Severity)
		}
	}
}

func TestHasCritical(t *testing.T) {
	tests := []struct {
		name     string
		critical int
		expected bool
	}{
		{"no critical", 0, false},
		{"has critical", 1, true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &models.ScanResult{
				Summary: models.Summary{Critical: tt.critical},
			}
			if got := result.HasCritical(); got != tt.expected {
				t.Errorf("HasCritical() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func BenchmarkScan(b *testing.B) {
	cfg := config.Default()
	s := New(cfg)
	for i := 0; i < b.N; i++ {
		s.Scan("all", "all")
	}
}

func TestScanInvalidNamespace(t *testing.T) {
	cfg := config.Default()
	s := New(cfg)
	
	result, err := s.Scan("nonexistent-namespace", "all")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	// Should return empty result for non-matching namespace
	_ = result
}

func TestScanWithHighFilter(t *testing.T) {
	cfg := config.Default()
	s := New(cfg)
	
	result, _ := s.Scan("all", "high")
	for _, issue := range result.Issues {
		if issue.Severity != "high" && issue.Severity != "critical" {
			t.Errorf("Expected high+, got %s", issue.Severity)
		}
	}
}

func TestScanWithMediumFilter(t *testing.T) {
	cfg := config.Default()
	s := New(cfg)
	
	result, _ := s.Scan("all", "medium")
	// Should return all issues since "medium" is subset
	_ = result
}

func TestCountSeverity(t *testing.T) {
	issues := []models.Issue{
		{Severity: "critical"},
		{Severity: "critical"},
		{Severity: "high"},
		{Severity: "medium"},
		{Severity: "low"},
	}
	
	tests := []struct {
		severity string
		expected int
	}{
		{"critical", 2},
		{"high", 1},
		{"medium", 1},
		{"low", 1},
		{"nonexistent", 0},
	}
	
	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			count := countSeverity(issues, tt.severity)
			if count != tt.expected {
				t.Errorf("countSeverity(%s) = %d, want %d", tt.severity, count, tt.expected)
			}
		})
	}
}

func TestPrintResults(t *testing.T) {
	cfg := config.Default()
	s := New(cfg)
	result, _ := s.Scan("all", "all")
	s.PrintResults(result)
}

func TestPrintJSON(t *testing.T) {
	cfg := config.Default()
	s := New(cfg)
	result, _ := s.Scan("all", "all")
	s.PrintJSON(result)
}

func TestScanResultSummary(t *testing.T) {
	cfg := config.Default()
	s := New(cfg)
	
	result, _ := s.Scan("all", "all")
	
	if result.Summary.TotalIssues != len(result.Issues) {
		t.Errorf("Summary.TotalIssues = %d, Issues = %d", 
			result.Summary.TotalIssues, len(result.Issues))
	}
	
	total := result.Summary.Critical + result.Summary.High + 
		result.Summary.Medium + result.Summary.Low
	if total != result.Summary.TotalIssues {
		t.Errorf("Sum = %d, Total = %d", total, result.Summary.TotalIssues)
	}
}
