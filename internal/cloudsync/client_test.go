package cloudsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fakeRunner struct {
	t        *testing.T
	calls    [][]string
	outputs  []string
	errs     []error
	callIdx  int
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if f.callIdx >= len(f.outputs) {
		f.t.Fatalf("unexpected call to Run: %s %v", name, args)
	}
	f.calls = append(f.calls, append([]string{name}, args...))
	out := f.outputs[f.callIdx]
	err := f.errs[f.callIdx]
	f.callIdx++
	return []byte(out), err
}

func TestClient_Version(t *testing.T) {
	runner := &fakeRunner{
		t:       t,
		outputs: []string{"rclone v1.68.2"},
		errs:    []error{nil},
	}
	client := NewRcloneClient(runner, "rclone", "/config.conf", "foo", 0)
	v, err := client.Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != "rclone v1.68.2" {
		t.Fatalf("expected rclone v1.68.2, got %q", v)
	}
	if !reflect.DeepEqual(runner.calls[0], []string{"rclone", "--config", "/config.conf", "version"}) {
		t.Fatalf("bad args: %v", runner.calls[0])
	}
}

func TestClient_RemoteExists(t *testing.T) {
	runner := &fakeRunner{
		t:       t,
		outputs: []string{"foo:\nmyfoo:\nbar:\n", "myfoo:\nbar:\n"},
		errs:    []error{nil, nil},
	}
	client := NewRcloneClient(runner, "rclone", "/config.conf", "foo", 0)
	
	exists, err := client.RemoteExists(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatalf("expected foo to exist")
	}

	exists, err = client.RemoteExists(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatalf("expected foo to NOT exist")
	}
}

func TestClient_Copy(t *testing.T) {
	runner := &fakeRunner{
		t:       t,
		outputs: []string{"", ""},
		errs:    []error{nil, errors.New("boom")},
	}
	client := NewRcloneClient(runner, "rclone", "/config.conf", "foo", 10)
	
	err := client.Copy(context.Background(), "/local/x.mp3", "Shakedown/2024/2024-01-02/My Show.mp3")
	if err != nil {
		t.Fatal(err)
	}
	
	expected := []string{"rclone", "--config", "/config.conf", "copyto", "/local/x.mp3", "foo:Shakedown/2024/2024-01-02/My Show.mp3", "--tpslimit", "10"}
	if !reflect.DeepEqual(runner.calls[0], expected) {
		t.Fatalf("bad args: %v", runner.calls[0])
	}

	err = client.Copy(context.Background(), "/local/x.mp3", "Shakedown/2024/2024-01-02/My Show.mp3")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "rclone copyto failed") {
		t.Fatalf("bad error message: %v", err)
	}
	if strings.Contains(err.Error(), "boom") {
		t.Fatalf("error should be sanitized: %v", err)
	}
}

func TestClient_StatSize(t *testing.T) {
	runner := &fakeRunner{
		t:       t,
		outputs: []string{
			`{"Size": 12345, "IsDir": false}`,
			`{"Size": 0, "IsDir": true}`,
			`[]`,
			`malformed`,
		},
		errs: []error{nil, nil, nil, nil},
	}
	client := NewRcloneClient(runner, "rclone", "/config.conf", "foo", 0)
	
	size, found, err := client.StatSize(context.Background(), "path/1.mp3")
	if err != nil { t.Fatal(err) }
	if !found || size != 12345 { t.Fatalf("bad result 1: %v %v", size, found) }

	size, found, err = client.StatSize(context.Background(), "path/dir")
	if err != nil { t.Fatal(err) }
	if found { t.Fatalf("expected dir to be not found: %v", size) }

	size, found, err = client.StatSize(context.Background(), "path/empty")
	if err != nil { t.Fatal(err) }
	if found { t.Fatalf("expected empty array to be not found: %v", size) }

	size, found, err = client.StatSize(context.Background(), "path/malformed")
	if err != nil { t.Fatal(err) }
	if found { t.Fatalf("expected malformed to be not found: %v", size) }
	
	expected := []string{"rclone", "--config", "/config.conf", "lsjson", "--stat", "foo:path/1.mp3"}
	if !reflect.DeepEqual(runner.calls[0], expected) {
		t.Fatalf("bad args: %v", runner.calls[0])
	}
}

func TestClient_WriteRemoteConfig(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "rclone.conf")
	client := NewRcloneClient(nil, "rclone", confPath, "foo", 0)
	
	validBlock := "[foo]\ntype = drive\nscope = drive\n"
	err := client.WriteRemoteConfig(context.Background(), validBlock)
	if err != nil {
		t.Fatal(err)
	}
	
	info, err := os.Stat(confPath)
	if err != nil { t.Fatal(err) }
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected 0600 mode, got %v", info.Mode().Perm())
	}
	
	content, _ := os.ReadFile(confPath)
	if string(content) != validBlock {
		t.Fatalf("bad content")
	}
	
	err = client.WriteRemoteConfig(context.Background(), "[wrong]\ntype=drive\n")
	if err == nil {
		t.Fatal("expected error for wrong section")
	}
}
