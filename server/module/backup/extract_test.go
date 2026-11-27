package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

func archive(t *testing.T, entries ...tar.Header) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, h := range entries {
		h := h
		if h.Typeflag == 0 {
			h.Typeflag = tar.TypeReg
		}
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			tw.Write(bytes.Repeat([]byte("x"), int(h.Size)))
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestExtractRejectsUnsafeArchives(t *testing.T) {
	cases := map[string][]tar.Header{
		"traversal":   {{Name: "media/../../evil", Size: 1}},
		"absolute":    {{Name: "/etc/passwd", Size: 1}},
		"symlink":     {{Name: "media/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}},
		"unexpected":  {{Name: "random.txt", Size: 1}},
		"no-manifest": {{Name: DBFile, Size: 1}},
	}
	for name, entries := range cases {
		dest := t.TempDir()
		if _, err := Extract(bytes.NewReader(archive(t, entries...)), dest, 1<<20); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, err := Extract(strings.NewReader("garbage"), t.TempDir(), 1<<20); err == nil {
		t.Error("garbage: expected error")
	}
	// Size bomb guard.
	if _, err := Extract(bytes.NewReader(archive(t, tar.Header{Name: DBFile, Size: 2048})), t.TempDir(), 1024); err == nil ||
		!strings.Contains(err.Error(), "allowed size") {
		t.Errorf("size limit: %v", err)
	}
}

func TestValidName(t *testing.T) {
	for name, want := range map[string]bool{
		"holocron-backup-20260101-120000.tar.gz": true, "pre-restore-x.tar.gz": true,
		"../x.tar.gz": false, ".hidden.tar.gz": false, "a/b.tar.gz": false, "x.zip": false,
	} {
		if ValidName(name) != want {
			t.Errorf("ValidName(%q) != %v", name, want)
		}
	}
}
