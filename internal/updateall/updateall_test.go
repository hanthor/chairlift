package updateall

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func collect(events <-chan Event) []Event {
	var received []Event
	for event := range events {
		received = append(received, event)
	}
	return received
}

// runWith executes a plan and returns both the results and every event, with
// a receiver draining the channel so emit never drops.
func runWith(t *testing.T, runner Runner, plan []Phase) ([]Result, []Event) {
	t.Helper()

	events := make(chan Event, 64)
	done := make(chan []Event, 1)
	go func() { done <- collect(events) }()

	results := runner.Run(context.Background(), plan, events)
	return results, <-done
}

func TestPhasesAreOrderedAndNotAliased(t *testing.T) {
	list := Phases()
	want := []PhaseID{PhaseOS, PhaseFlatpak, PhaseBrew}
	if len(list) != len(want) {
		t.Fatalf("Phases() has %d entries, want %d", len(list), len(want))
	}
	for index, phase := range list {
		if phase.ID != want[index] {
			t.Errorf("Phases()[%d].ID = %q, want %q", index, phase.ID, want[index])
		}
		if phase.Title == "" {
			t.Errorf("Phases()[%d] (%q) has no title", index, phase.ID)
		}
	}

	list[0].Title = "mutated"
	if Phases()[0].Title == "mutated" {
		t.Error("Phases() returned an aliased slice")
	}
}

// A provider that is not installed is not a failure — it never enters the
// plan, matching how ChairLift hides a group whose tool is absent.
func TestPlanIncludesOnlyAvailableProviders(t *testing.T) {
	tests := []struct {
		name         string
		availability Availability
		want         []PhaseID
	}{
		{
			name:         "everything available",
			availability: Availability{OS: true, Flatpak: true, Brew: true},
			want:         []PhaseID{PhaseOS, PhaseFlatpak, PhaseBrew},
		},
		{
			name:         "no bootc host",
			availability: Availability{Flatpak: true, Brew: true},
			want:         []PhaseID{PhaseFlatpak, PhaseBrew},
		},
		{
			name:         "os only",
			availability: Availability{OS: true},
			want:         []PhaseID{PhaseOS},
		},
		{
			name:         "brew only keeps execution order",
			availability: Availability{Brew: true},
			want:         []PhaseID{PhaseBrew},
		},
		{
			name:         "nothing available",
			availability: Availability{},
			want:         nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := Plan(test.availability)
			got := make([]PhaseID, 0, len(plan))
			for _, phase := range plan {
				got = append(got, phase.ID)
			}
			if len(got) == 0 && len(test.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("Plan(%+v) = %v, want %v", test.availability, got, test.want)
			}
		})
	}
}

func TestAvailableRejectsUnknownPhase(t *testing.T) {
	availability := Availability{OS: true, Flatpak: true, Brew: true}
	if availability.Available(PhaseID("distrobox")) {
		t.Error("Available() accepted a phase that is not in the inventory")
	}
}

func okRunner() Runner {
	return Runner{
		StageOS:       func(context.Context, func(string)) error { return nil },
		StagedAfter:   func(context.Context) (bool, string) { return true, "42.20260817" },
		UpdateFlatpak: func(context.Context) error { return nil },
		UpdateBrew:    func(context.Context) error { return nil },
	}
}

func TestRunExecutesEveryPhaseAndEmitsPairedEvents(t *testing.T) {
	var order []PhaseID
	runner := okRunner()
	runner.StageOS = func(_ context.Context, emitLine func(string)) error {
		order = append(order, PhaseOS)
		emitLine("Pulling layer 1")
		emitLine("Staging")
		return nil
	}
	runner.UpdateFlatpak = func(context.Context) error { order = append(order, PhaseFlatpak); return nil }
	runner.UpdateBrew = func(context.Context) error { order = append(order, PhaseBrew); return nil }

	results, events := runWith(t, runner, Plan(Availability{OS: true, Flatpak: true, Brew: true}))

	if !reflect.DeepEqual(order, []PhaseID{PhaseOS, PhaseFlatpak, PhaseBrew}) {
		t.Errorf("phases ran in order %v, want os, flatpak, brew", order)
	}
	if len(results) != 3 {
		t.Fatalf("Run() returned %d results, want 3", len(results))
	}
	for _, result := range results {
		if result.Outcome != OutcomeSucceeded {
			t.Errorf("%s outcome = %q, want %q", result.Phase.ID, result.Outcome, OutcomeSucceeded)
		}
	}

	// Every phase must produce exactly one started and one finished event, or
	// a UI driven by these events would leave a row spinning forever.
	started, finished, messages := 0, 0, 0
	for _, event := range events {
		switch event.Type {
		case EventPhaseStarted:
			started++
		case EventPhaseFinished:
			finished++
		case EventMessage:
			messages++
		}
	}
	if started != 3 || finished != 3 {
		t.Errorf("events: %d started, %d finished, want 3 and 3", started, finished)
	}
	if messages != 2 {
		t.Errorf("streamed messages = %d, want 2", messages)
	}
}

// The OS image is independent of Flatpak and Homebrew, so a failed system
// stage must not leave a user's applications un-updated as well.
func TestRunContinuesAfterAPhaseFails(t *testing.T) {
	flatpakRan, brewRan := false, false
	runner := okRunner()
	runner.StageOS = func(context.Context, func(string)) error { return errors.New("registry unreachable") }
	runner.UpdateFlatpak = func(context.Context) error { flatpakRan = true; return nil }
	runner.UpdateBrew = func(context.Context) error { brewRan = true; return nil }

	results, _ := runWith(t, runner, Plan(Availability{OS: true, Flatpak: true, Brew: true}))

	if !flatpakRan || !brewRan {
		t.Error("a failed OS phase aborted the independent phases")
	}
	if results[0].Outcome != OutcomeFailed {
		t.Errorf("OS outcome = %q, want %q", results[0].Outcome, OutcomeFailed)
	}
	if !strings.Contains(results[0].Detail, "registry unreachable") {
		t.Errorf("OS detail = %q, want the underlying error preserved", results[0].Detail)
	}
	for _, result := range results[1:] {
		if result.Outcome != OutcomeSucceeded {
			t.Errorf("%s outcome = %q, want it unaffected by the OS failure", result.Phase.ID, result.Outcome)
		}
	}
}

// A nil seam is a wiring mistake. It must not report the system as updated.
func TestRunTreatsAMissingSeamAsFailureNotSuccess(t *testing.T) {
	results, _ := runWith(t, Runner{}, Plan(Availability{OS: true, Flatpak: true, Brew: true}))

	for _, result := range results {
		if result.Outcome != OutcomeFailed {
			t.Errorf("%s outcome = %q with no seam wired, want %q", result.Phase.ID, result.Outcome, OutcomeFailed)
		}
	}
	if Summarize(results).RestartRequired {
		t.Error("a run with no seams wired asked for a restart")
	}
}

func TestRunSkipsRemainingPhasesOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	brewRan := false

	runner := okRunner()
	runner.StageOS = func(context.Context, func(string)) error { cancel(); return nil }
	runner.UpdateBrew = func(context.Context) error { brewRan = true; return nil }

	events := make(chan Event, 64)
	go func() {
		for range events {
		}
	}()
	results := runner.Run(ctx, Plan(Availability{OS: true, Flatpak: true, Brew: true}), events)

	if brewRan {
		t.Error("Homebrew ran after the run was cancelled")
	}
	if len(results) != 3 {
		t.Fatalf("Run() returned %d results, want one per planned phase even when cancelled", len(results))
	}
	for _, result := range results[1:] {
		if result.Outcome != OutcomeSkipped {
			t.Errorf("%s outcome = %q after cancellation, want %q", result.Phase.ID, result.Outcome, OutcomeSkipped)
		}
	}
}

// The staging script is idempotent and exits 0 when the system is already
// current, so its error is not evidence that anything was staged.
func TestOSPhaseDistinguishesStagedFromAlreadyCurrent(t *testing.T) {
	tests := []struct {
		name        string
		staged      bool
		version     string
		wantDetail  string
		wantRestart bool
	}{
		{name: "staged with a version", staged: true, version: "42.20260817", wantDetail: "Update 42.20260817 staged", wantRestart: true},
		{name: "staged without a version", staged: true, wantDetail: "Update staged", wantRestart: true},
		{name: "already current", staged: false, wantDetail: "Already up to date", wantRestart: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := okRunner()
			runner.StagedAfter = func(context.Context) (bool, string) { return test.staged, test.version }

			results, _ := runWith(t, runner, Plan(Availability{OS: true}))
			if results[0].Outcome != OutcomeSucceeded {
				t.Fatalf("OS outcome = %q, want %q", results[0].Outcome, OutcomeSucceeded)
			}
			if !strings.Contains(results[0].Detail, test.wantDetail) {
				t.Errorf("OS detail = %q, want it to contain %q", results[0].Detail, test.wantDetail)
			}
			if got := Summarize(results).RestartRequired; got != test.wantRestart {
				t.Errorf("RestartRequired = %v, want %v", got, test.wantRestart)
			}
		})
	}
}

// A missing StagedAfter seam must not be read as "staged".
func TestOSPhaseWithoutAStagedProbeDoesNotClaimARestartIsNeeded(t *testing.T) {
	runner := okRunner()
	runner.StagedAfter = nil

	results, _ := runWith(t, runner, Plan(Availability{OS: true}))
	if Summarize(results).RestartRequired {
		t.Error("RestartRequired = true with no staged probe wired, want false")
	}
}

func TestSummarizeCountsAndHeadlines(t *testing.T) {
	os := Phase{ID: PhaseOS, Title: "System Image"}
	flatpak := Phase{ID: PhaseFlatpak, Title: "Applications"}
	brew := Phase{ID: PhaseBrew, Title: "Homebrew Packages"}

	tests := []struct {
		name         string
		results      []Result
		wantHeadline string
		wantRestart  bool
		wantFailed   []string
	}{
		{
			name:         "nothing planned",
			results:      nil,
			wantHeadline: "Nothing to update",
		},
		{
			name: "all current",
			results: []Result{
				{Phase: os, Outcome: OutcomeSucceeded, Detail: "Already up to date"},
				{Phase: flatpak, Outcome: OutcomeSucceeded, Detail: "Up to date"},
			},
			wantHeadline: "Everything is up to date",
		},
		{
			name: "restart pending",
			results: []Result{
				{Phase: os, Outcome: OutcomeSucceeded, Detail: "Update 42 staged — restart to apply"},
				{Phase: flatpak, Outcome: OutcomeSucceeded, Detail: "Up to date"},
			},
			wantHeadline: "Update complete — restart to apply",
			wantRestart:  true,
		},
		{
			name: "everything failed",
			results: []Result{
				{Phase: os, Outcome: OutcomeFailed, Detail: "boom"},
				{Phase: flatpak, Outcome: OutcomeFailed, Detail: "boom"},
			},
			wantHeadline: "Update failed",
			wantFailed:   []string{"System Image", "Applications"},
		},
		{
			name: "partial failure without a staged image",
			results: []Result{
				{Phase: os, Outcome: OutcomeSucceeded, Detail: "Already up to date"},
				{Phase: brew, Outcome: OutcomeFailed, Detail: "brew exploded"},
			},
			wantHeadline: "Updated with 1 problem(s)",
			wantFailed:   []string{"Homebrew Packages"},
		},
		{
			name: "partial failure with a staged image still asks for a restart",
			results: []Result{
				{Phase: os, Outcome: OutcomeSucceeded, Detail: "Update 42 staged — restart to apply"},
				{Phase: brew, Outcome: OutcomeFailed, Detail: "brew exploded"},
			},
			wantHeadline: "restart to apply the system image",
			wantRestart:  true,
			wantFailed:   []string{"Homebrew Packages"},
		},
		{
			name: "cancelled",
			results: []Result{
				{Phase: os, Outcome: OutcomeSucceeded, Detail: "Already up to date"},
				{Phase: flatpak, Outcome: OutcomeSkipped, Detail: "Cancelled"},
			},
			wantHeadline: "Update cancelled",
		},
		{
			// A failed OS phase never staged anything, so no restart.
			name: "failed os phase does not ask for a restart",
			results: []Result{
				{Phase: os, Outcome: OutcomeFailed, Detail: "staged nothing because it failed"},
			},
			wantHeadline: "Update failed",
			wantFailed:   []string{"System Image"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			summary := Summarize(test.results)
			if !strings.Contains(summary.Headline, test.wantHeadline) {
				t.Errorf("Headline = %q, want it to contain %q", summary.Headline, test.wantHeadline)
			}
			if summary.RestartRequired != test.wantRestart {
				t.Errorf("RestartRequired = %v, want %v", summary.RestartRequired, test.wantRestart)
			}
			if test.wantFailed != nil && !reflect.DeepEqual(summary.FailedPhases, test.wantFailed) {
				t.Errorf("FailedPhases = %v, want %v", summary.FailedPhases, test.wantFailed)
			}
			if total := summary.Succeeded + summary.Failed + summary.Skipped; total != len(test.results) {
				t.Errorf("counts sum to %d, want %d", total, len(test.results))
			}
		})
	}
}

func TestPhaseTitlesAreSorted(t *testing.T) {
	got := PhaseTitles(Phases())
	want := []string{"Applications", "Homebrew Packages", "System Image"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PhaseTitles() = %v, want %v", got, want)
	}
}

func TestRunClosesTheEventChannel(t *testing.T) {
	events := make(chan Event, 8)
	okRunner().Run(context.Background(), Plan(Availability{Brew: true}), events)

	// Ranging terminates only if the channel was closed; a UI driven by
	// `for event := range events` would otherwise hang forever.
	for range events {
	}
}

// An abandoned UI must not wedge the update. The timeout is what makes this a
// failure rather than a hung test run.
func TestRunDoesNotBlockOnAnUndrainedEventChannel(t *testing.T) {
	events := make(chan Event) // unbuffered, never read
	done := make(chan []Result, 1)

	go func() {
		done <- okRunner().Run(context.Background(), Plan(Availability{OS: true, Flatpak: true, Brew: true}), events)
	}()

	select {
	case results := <-done:
		if len(results) != 3 {
			t.Errorf("Run() returned %d results, want 3", len(results))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run() blocked on an event channel with no receiver")
	}
}
