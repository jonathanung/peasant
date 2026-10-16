package auth

import (
	"bufio"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed testdata/credential_observations.yaml
var credentialObservationFixtures []byte

type credentialFixtures struct {
	Generation []struct {
		Name    string   `yaml:"name"`
		Actions []string `yaml:"actions"`
		Changed bool     `yaml:"changed"`
	} `yaml:"generation"`
	URLs []struct {
		Name       string `yaml:"name"`
		Input      string `yaml:"input"`
		Normalized string `yaml:"normalized"`
		Invalid    bool   `yaml:"invalid"`
	} `yaml:"urls"`
}

func loadCredentialFixtures(t *testing.T) credentialFixtures {
	t.Helper()
	var fixtures credentialFixtures
	if err := yaml.Unmarshal(credentialObservationFixtures, &fixtures); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, fixture := range fixtures.Generation {
		if fixture.Name == "" || names[fixture.Name] {
			t.Fatal("missing/duplicate fixture name")
		}
		names[fixture.Name] = true
	}
	for _, fixture := range fixtures.URLs {
		if fixture.Name == "" || names[fixture.Name] {
			t.Fatal("missing/duplicate fixture name")
		}
		names[fixture.Name] = true
	}
	const required = "unchanged-generation-commits credential-rotation-refuses-old-response account-switch-back-refuses-old-response logout-refuses-old-response logout-login-same-account-refuses-old-response full-base-path-preserved same-host-different-base-path url-secrets-not-scope nondefault-port-preserved explicit-loopback-development ipv6-default-port insecure-remote-refused relative-url-refused ambiguous-parent-path-refused encoded-separator-refused ambiguous-empty-path-segment-refused unsupported-scheme-refused"
	for _, name := range strings.Fields(required) {
		if !names[name] {
			t.Fatalf("required fixture %s missing", name)
		}
	}
	return fixtures
}

func observationCredentials() Credentials {
	return Credentials{APIKey: "fixture-key", KeyID: "fixture-key-id", UserID: "11111111-1111-4111-8111-111111111111", Username: "fixture", VillageURL: "https://village.example/team"}
}

func TestCredentialSnapshotGeneration(t *testing.T) {
	for _, fixture := range loadCredentialFixtures(t).Generation {
		t.Run(fixture.Name, func(t *testing.T) {
			home := t.TempDir()
			creds := observationCredentials()
			if err := SaveCredentialsFrom(&creds, home); err != nil {
				t.Fatal(err)
			}
			snapshot, err := CaptureCredentials(context.Background(), home)
			if err != nil {
				t.Fatal(err)
			}
			for _, action := range fixture.Actions {
				switch action {
				case "save-same":
					err = SaveCredentialsFrom(&creds, home)
				case "save-other":
					other := creds
					other.UserID = "22222222-2222-4222-8222-222222222222"
					err = SaveCredentialsFrom(&other, home)
				case "clear":
					err = ClearCredentialsFrom(home)
				default:
					t.Fatalf("unknown action %s", action)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			committed := false
			err = snapshot.WithCurrent(context.Background(), func() error { committed = true; return nil })
			if fixture.Changed {
				if !errors.Is(err, ErrCredentialsChanged) || committed {
					t.Fatalf("obsolete generation committed=%v err=%v", committed, err)
				}
			} else if err != nil || !committed {
				t.Fatalf("current generation committed=%v err=%v", committed, err)
			}
		})
	}
}

func TestNormalizeVillageURL(t *testing.T) {
	for _, fixture := range loadCredentialFixtures(t).URLs {
		t.Run(fixture.Name, func(t *testing.T) {
			got, err := NormalizeVillageURL(fixture.Input)
			if fixture.Invalid {
				if err == nil {
					t.Fatal("ambiguous scope accepted")
				}
				return
			}
			if err != nil || got != fixture.Normalized {
				t.Fatalf("normalized=%q want=%q err=%v", got, fixture.Normalized, err)
			}
		})
	}
}

// A real child process must contend on the OS lock, not an in-memory mutex.
// Its deadline expires while the parent's transaction callback is active.
func TestCredentialCommitGuardCrossProcess(t *testing.T) {
	if home := os.Getenv("PEASANT_CREDENTIAL_GUARD_CHILD"); home != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_, err := CaptureCredentials(ctx, home)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("cross-process lock did not protect commit: %v", err)
		}
		fmt.Println("deadline-proved-lock")
		return
	}
	home := t.TempDir()
	creds := observationCredentials()
	if err := SaveCredentialsFrom(&creds, home); err != nil {
		t.Fatal(err)
	}
	snapshot, err := CaptureCredentials(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = snapshot.WithCurrent(ctx, func() error {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCredentialCommitGuardCrossProcess$")
		cmd.Env = append(os.Environ(), "PEASANT_CREDENTIAL_GUARD_CHILD="+home)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("child: %w: %s", err, out)
		}
		if !strings.Contains(string(out), "deadline-proved-lock") {
			return fmt.Errorf("child did not prove contention: %s", out)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCredentialMutationWaitsForCommit(t *testing.T) {
	if home := os.Getenv("PEASANT_CREDENTIAL_SAVE_CHILD"); home != "" {
		fmt.Println("ready")
		if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
			t.Fatal(err)
		}
		creds := observationCredentials()
		if err := SaveCredentialsFrom(&creds, home); err != nil {
			t.Fatal(err)
		}
		return
	}
	home := t.TempDir()
	creds := observationCredentials()
	if err := SaveCredentialsFrom(&creds, home); err != nil {
		t.Fatal(err)
	}
	snapshot, err := CaptureCredentials(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCredentialMutationWaitsForCommit$")
	cmd.Env = append(os.Environ(), "PEASANT_CREDENTIAL_SAVE_CHILD="+home)
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = in.Close(); _ = cmd.Process.Kill() }()
	if line, err := bufio.NewReader(out).ReadString('\n'); err != nil || line != "ready\n" {
		t.Fatalf("child startup: %q %v", line, err)
	}
	done := make(chan error, 1)
	if err := snapshot.WithCurrent(ctx, func() error {
		if _, err := fmt.Fprintln(in, "save"); err != nil {
			return err
		}
		go func() { done <- cmd.Wait() }()
		// Wait is bounded to prove the real save cannot finish before commit.
		select {
		case err := <-done:
			return fmt.Errorf("save escaped active commit: %v", err)
		case <-time.After(100 * time.Millisecond):
			return nil
		}
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := snapshot.WithCurrent(ctx, func() error { t.Fatal("old generation installed after concurrent save"); return nil }); !errors.Is(err, ErrCredentialsChanged) {
		t.Fatalf("generation was not rotated: %v", err)
	}
}

func TestCredentialSnapshotSignedOut(t *testing.T) {
	_, err := CaptureCredentials(context.Background(), t.TempDir())
	if !errors.Is(err, ErrSignedOut) {
		t.Fatalf("signed-out scope: %v", err)
	}
}
