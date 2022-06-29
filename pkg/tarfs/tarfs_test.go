package tarfs

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"
	"testing"
	"testing/fstest"
)

// TestFS runs some sanity checks on a tar generated from this package's
// directory.
//
// The tar is generated on demand and removed if tests fail, so modifying any
// file in this package *will* cause tests to fail once. Make sure to run tests
// twice if the Checksum tests fail.
func TestFS(t *testing.T) {
	const name = `testdata/fstest.tar`
	checktar(t, name)
	fileset := []string{
		"file.go",
		"parse.go",
		"tarfs.go",
		"tarfs_test.go",
		"testdata/.gitignore",
	}

	t.Run("Single", func(t *testing.T) {
		f, err := os.Open(name)
		if err != nil {
			t.Error(err)
		}
		t.Cleanup(func() {
			if err := f.Close(); err != nil {
				t.Error(err)
			}
		})
		sys, err := New(f)
		if err != nil {
			t.Error(err)
		}

		if err := fstest.TestFS(sys, fileset...); err != nil {
			t.Error(err)
		}
	})

	t.Run("Concurrent", func(t *testing.T) {
		f, err := os.Open(name)
		if err != nil {
			t.Error(err)
		}
		t.Cleanup(func() {
			if err := f.Close(); err != nil {
				t.Error(err)
			}
		})
		sys, err := New(f)
		if err != nil {
			t.Error(err)
		}

		const lim = 8
		var wg sync.WaitGroup
		t.Logf("running %d goroutines", lim)
		wg.Add(lim)
		for i := 0; i < lim; i++ {
			go func() {
				defer wg.Done()
				if err := fstest.TestFS(sys, fileset...); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
	})

	t.Run("Sub", func(t *testing.T) {
		f, err := os.Open(name)
		if err != nil {
			t.Error(err)
		}
		t.Cleanup(func() {
			if err := f.Close(); err != nil {
				t.Error(err)
			}
		})
		sys, err := New(f)
		if err != nil {
			t.Error(err)
		}

		sub, err := fs.Sub(sys, "testdata")
		if err != nil {
			t.Error(err)
		}
		if err := fstest.TestFS(sub, ".gitignore"); err != nil {
			t.Error(err)
		}
	})

	t.Run("Checksum", func(t *testing.T) {
		f, err := os.Open(name)
		if err != nil {
			t.Error(err)
		}
		t.Cleanup(func() {
			if err := f.Close(); err != nil {
				t.Error(err)
			}
		})
		sys, err := New(f)
		if err != nil {
			t.Error(err)
		}
		for _, n := range fileset {
			name := n
			t.Run(name, func(t *testing.T) {
				h := sha256.New()
				f, err := os.Open(name)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				if _, err := io.Copy(h, f); err != nil {
					t.Error(err)
				}
				want := h.Sum(nil)

				h.Reset()
				b, err := fs.ReadFile(sys, name)
				if err != nil {
					t.Error(err)
				}
				if _, err := h.Write(b); err != nil {
					t.Error(err)
				}
				got := h.Sum(nil)

				if !bytes.Equal(got, want) {
					t.Errorf("got: %x, want: %x", got, want)
				}
			})
		}
	})
}

// TestEmpty tests that a wholly empty tar still creates an empty root.
func TestEmpty(t *testing.T) {
	// Two zero blocks is the tar footer, so just make one up.
	rd := bytes.NewReader(make([]byte, 2*512))
	sys, err := New(rd)
	if err != nil {
		t.Error(err)
	}
	if _, err := fs.Stat(sys, "."); err != nil {
		t.Error(err)
	}
	ent, err := fs.ReadDir(sys, ".")
	if err != nil {
		t.Error(err)
	}
	for _, e := range ent {
		t.Log(e)
	}
	if len(ent) != 0 {
		t.Errorf("got: %d, want: 0", len(ent))
	}
}

func checktar(t *testing.T, name string) {
	t.Helper()
	out, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	tw := tar.NewWriter(out)
	defer tw.Close()

	in := os.DirFS(".")
	if err := fs.WalkDir(in, ".", mktar(t, in, tw)); err != nil {
		t.Fatal(err)
	}
}

func mktar(t *testing.T, in fs.FS, tw *tar.Writer) fs.WalkDirFunc {
	return func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case d.Name() == "fstest.tar":
			return nil
		case d.Name() == "." && d.IsDir():
			return nil
		default:
		}
		t.Logf("adding %q", p)
		i, err := d.Info()
		if err != nil {
			return err
		}
		h, err := tar.FileInfoHeader(i, "")
		if err != nil {
			return err
		}
		h.Name = p
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if i.IsDir() {
			return nil
		}
		f, err := in.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(tw, f); err != nil {
			return err
		}
		return nil
	}
}

func TestSymlinks(t *testing.T) {
	tmp := t.TempDir()
	run := func(wantErr bool, hs []tar.Header) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()
			f, err := os.Create(filepath.Join(tmp, path.Base(t.Name())))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			tw := tar.NewWriter(f)
			for i := range hs {
				if err := tw.WriteHeader(&hs[i]); err != nil {
					t.Error(err)
				}
			}
			if err := tw.Close(); err != nil {
				t.Error(err)
			}

			_, err = New(f)
			t.Log(err)
			if (err != nil) != wantErr {
				t.Fail()
			}
		}
	}
	t.Run("Ordered", run(false, []tar.Header{
		{Name: `a/`},
		{
			Typeflag: tar.TypeSymlink,
			Name:     `b`,
			Linkname: `a`,
		},
		{Name: `b/c`},
	}))
	t.Run("Unordered", run(false, []tar.Header{
		{
			Typeflag: tar.TypeSymlink,
			Name:     `b`,
			Linkname: `a`,
		},
		{Name: `b/c`},
		{Name: `a/`},
	}))
	t.Run("LinkToReg", run(true, []tar.Header{
		{Name: `a`},
		{
			Typeflag: tar.TypeSymlink,
			Name:     `b`,
			Linkname: `a`,
		},
		{Name: `b/c`},
	}))
	t.Run("UnorderedLinkToReg", run(true, []tar.Header{
		{
			Typeflag: tar.TypeSymlink,
			Name:     `b`,
			Linkname: `a`,
		},
		{Name: `b/c`},
		{Name: `a`},
	}))
	t.Run("Cycle", run(true, []tar.Header{
		{
			Typeflag: tar.TypeSymlink,
			Name:     `b`,
			Linkname: `a`,
		},
		{
			Typeflag: tar.TypeSymlink,
			Name:     `a`,
			Linkname: `b`,
		},
		{Name: `b/c`},
	}))
}
