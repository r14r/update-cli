package rsync

import "testing"

func TestParseChanges(t *testing.T) {
	changes := ParseChanges(">f+++++++++ new.txt\n>f.st...... changed.txt\n*deleting   old.txt\n")
	if len(changes) != 3 {
		t.Fatalf("len = %d", len(changes))
	}
	if changes[0].Kind != ChangeCreated || changes[1].Kind != ChangeUpdated || changes[2].Kind != ChangeDeleted {
		t.Fatalf("unexpected changes: %#v", changes)
	}
}
