package urban

import "testing"

func TestEveryRuleDescribesItsContracts(t *testing.T) {
	for _, definition := range Plugin().Rules {
		description := definition.Description
		if err := description.Check(); err != nil {
			t.Errorf("rule %q: %v", description.Name, err)
		}
		if !description.Described() {
			t.Errorf("rule %q lists what it writes but does not describe it (publishes)", description.Name)
		}
	}
}
