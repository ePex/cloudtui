package devtool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestJavaBinaryUsesJavaHomeWhenSet(t *testing.T) {
	t.Setenv("JAVA_HOME", filepath.FromSlash("/opt/jdk-21"))
	want := filepath.Join("/opt/jdk-21", "bin", "java")
	if runtime.GOOS == "windows" {
		want = filepath.Join("/opt/jdk-21", "bin", "java.exe")
	}
	if got := javaBinary(); got != want {
		t.Errorf("javaBinary() = %q, want %q", got, want)
	}
}

func TestJavaBinaryFallsBackToPathWhenJavaHomeUnset(t *testing.T) {
	t.Setenv("JAVA_HOME", "")
	if got := javaBinary(); got != "java" {
		t.Errorf("javaBinary() = %q, want \"java\"", got)
	}
}

func TestWaitHTTPSucceedsImmediatelyWhenServerIsUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized) // even a non-2xx response means "up"
	}))
	defer srv.Close()

	start := time.Now()
	if err := waitHTTP(context.Background(), srv.URL, 5*time.Second); err != nil {
		t.Fatalf("waitHTTP() error = %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("waitHTTP() took %s for an already-up server, want near-instant", elapsed)
	}
}

func TestWaitHTTPTimesOutWhenNothingListens(t *testing.T) {
	err := waitHTTP(context.Background(), "http://127.0.0.1:1", 500*time.Millisecond)
	if err == nil {
		t.Fatal("waitHTTP() error = nil, want a timeout error when nothing listens")
	}
}

func TestWaitHTTPRespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := waitHTTP(ctx, "http://127.0.0.1:1", 5*time.Second)
	if err == nil {
		t.Fatal("waitHTTP() error = nil, want an error for an already-cancelled context")
	}
}
