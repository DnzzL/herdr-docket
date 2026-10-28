package daemon

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/DnzzL/herdr-docket/internal/grill"
	"github.com/DnzzL/herdr-docket/internal/host"
	"github.com/DnzzL/herdr-docket/internal/hostpath"
	"github.com/DnzzL/herdr-docket/internal/work"
)

// A card in Question is a debt: a grill round waits on the owner's answer, and
// nothing moves it until they give it. The board is where the debt is written
// down, not where it is heard, so the tick speaks it — one popup per round, in
// the `request` sound the merge a human owes already uses (ADR 0015).
//
// The marker is what keeps a 15-second tick from re-announcing a round
// forever, and what lets a restart (the daemon re-execs on upgrade) remember
// it. One small file per card and round, like the heartbeat; written only
// after the popup returns clean, so a refused one retries next tick. A crash
// between the two repeats a popup, and at-most-once is not on offer: a repeat
// is the cheaper error against an unhearable one (ADR 0015).
const questionMarkerPrefix = "question-"

// questionMarkerPath names one announcement. The card id's separator cannot
// ride a filename, so path separators become '_' — ids differ by more than a
// separator, so two cards cannot land on one marker.
func questionMarkerPath(id string, round int) string {
	safe := strings.NewReplacer("/", "_", `\`, "_").Replace(id)
	return filepath.Join(hostpath.StateDir(), fmt.Sprintf("%s%s-r%d", questionMarkerPrefix, safe, round))
}

// announceQuestions raises one popup for every open Question card whose newest
// round has not been announced yet, and forgets the announcements no card in
// Question still owes — a card that left Question is a debt paid, so returning
// to it on the same round speaks again and the state dir stops growing.
//
// The round comes from src.Get, never from the list row: a queue's list is a
// summary (Backlog.md's carries no notes at all), and only Get returns the
// notes a grill round is written in. Reading notes off List would make every
// card look roundless and every popup silently vanish.
//
// Best-effort by contract (ADR 0010): a failed read or notification is logged
// and retried next tick, never turned into a failed tick.
func announceQuestions(h host.Host, src work.Source, tasks []work.Task) {
	type owed struct {
		task  work.Task
		round int
	}
	var announcements []owed
	keep := map[string]bool{}
	for _, t := range tasks {
		if !t.Open || !t.Question {
			continue
		}
		card, err := src.Get(t.ID)
		if err != nil {
			log.Printf("%s: read the card: %v", t.ID, err)
			continue // no marker: an unread card is not an announced one
		}
		r, ok := grill.LatestRound(card.Notes)
		if !ok {
			continue // nothing a human could be asked to answer
		}
		announcements = append(announcements, owed{task: card, round: r.N})
		keep[questionMarkerPath(card.ID, r.N)] = true
	}

	forgetQuestionMarkers(keep)

	for _, a := range announcements {
		path := questionMarkerPath(a.task.ID, a.round)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		body := fmt.Sprintf("round %d — %s", a.round, a.task.Title)
		if err := h.Notify("answer owed — "+a.task.ID, body, host.SoundRequest); err != nil {
			log.Printf("%s: notify: %v", a.task.ID, err)
			continue
		}
		if err := writeQuestionMarker(path); err != nil {
			log.Printf("%s: record the announcement: %v", a.task.ID, err)
		}
	}
}

// forgetQuestionMarkers removes every marker not in keep. A missing state dir
// simply holds none.
func forgetQuestionMarkers(keep map[string]bool) {
	marks, err := filepath.Glob(filepath.Join(hostpath.StateDir(), questionMarkerPrefix+"*"))
	if err != nil {
		log.Printf("question markers: %v", err)
		return
	}
	for _, p := range marks {
		if keep[p] {
			continue
		}
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			log.Printf("question markers: remove %s: %v", filepath.Base(p), err)
		}
	}
}

func writeQuestionMarker(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, nil, 0o644)
}
