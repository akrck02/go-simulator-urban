package urban

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/akrck02/go-simulator/configuration"
	"github.com/akrck02/go-simulator/host"
)

// TestRulesFileRunsInProcess runs a real rules file with the plugin inside the test, to profile
// it: URBAN_RULES=path/to/rules.yaml URBAN_UNTIL=-25 go test -run RulesFile -cpuprofile cpu.out ./urban
func TestRulesFileRunsInProcess(t *testing.T) {
	path := os.Getenv("URBAN_RULES")
	if path == "" {
		t.Skip("set URBAN_RULES to a rules file")
	}
	settings, err := configuration.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for index := range settings.Plugins {
		settings.Plugins[index].Builtin = true
	}
	t.Setenv("GO_SIMULATOR_RULES_DIRECTORY", filepath.Dir(path))
	engine, err := host.Build(settings, nil, map[string]host.Builtin{"urban": Plugin().ServeConnection})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	if _, err := engine.Start(); err != nil {
		t.Fatal(err)
	}
	until := 1e9
	if written := os.Getenv("URBAN_UNTIL"); written != "" {
		if _, err := fmtSscan(written, &until); err != nil {
			t.Fatal(err)
		}
	}
	stage := engine.Stages[0]
	for !stage.Finished() && stage.Time < until {
		if _, err := engine.StepStage(stage.Name); err != nil {
			t.Fatal(err)
		}
	}
}

func fmtSscan(text string, value *float64) (int, error) { return fmt.Sscan(text, value) }
