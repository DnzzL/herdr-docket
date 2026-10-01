package backlogmd

import (
	"reflect"
	"testing"
)

// SetCriteria drives the CLI's replace-all criteria verb, one flag per
// criterion: the write side of the bar that Create can carry.
func TestSetCriteriaReplacesThroughTheCLI(t *testing.T) {
	f := &fakeRun{out: map[string]string{}}
	c := &cli{dir: "/tmp/x", run: f.run}
	if err := c.SetCriteria("TASK-9", []string{"compiles", "the bar is real"}); err != nil {
		t.Fatalf("SetCriteria: %v", err)
	}
	want := []string{"task", "edit", "TASK-9", "--plain", "--ac", "compiles", "--ac", "the bar is real"}
	if !reflect.DeepEqual(f.calls[0], want) {
		t.Fatalf("call = %v, want %v", f.calls[0], want)
	}
}
