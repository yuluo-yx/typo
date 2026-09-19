package storage

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestFileLockContentionAndRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.lock")
	wantErr := errors.New("transaction failure")
	err := WithFileLock(t.Context(), path, func() error {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
		defer cancel()
		err := WithFileLock(ctx, path, func() error {
			t.Error("contending transaction acquired the lock")
			return nil
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("contention error = %v", err)
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatal(err)
	}
	if err := WithFileLock(t.Context(), path, func() error { return nil }); err != nil {
		t.Fatalf("transaction failure leaked the lock: %v", err)
	}
}

func TestFileLockErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := WithFileLock(ctx, filepath.Join(t.TempDir(), "cancel.lock"), nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled lock = %v", err)
	}
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(blocker, "lock"), filepath.Dir(blocker)} {
		if err := WithFileLock(t.Context(), path, nil); err == nil {
			t.Fatalf("expected lock path error for %s", path)
		}
	}
	file, err := os.Open(blocker)
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	if err := waitForFileLock(t.Context(), file); err == nil {
		t.Fatal("closed descriptor unexpectedly acquired a lock")
	}
}

func TestFileLockReleasedWhenProcessExits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.lock")
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestFileLockHelperProcess$")
	cmd.Env = append(os.Environ(), "TYPO_TEST_LOCK_PATH="+path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || line != "locked\n" {
		t.Fatalf("helper did not acquire lock: %q, %v", line, err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if err := WithFileLock(ctx, path, func() error { return nil }); err != nil {
		t.Fatalf("dead process retained the lock: %v", err)
	}
}

func TestFileLockHelperProcess(t *testing.T) {
	path := os.Getenv("TYPO_TEST_LOCK_PATH")
	if path == "" {
		return
	}
	err := WithFileLock(t.Context(), path, func() error {
		fmt.Println("locked")
		_, err := os.Stdin.Read(make([]byte, 1))
		return err
	})
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
