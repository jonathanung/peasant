package pull

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/peasant-labs/peasant/internal/ingest"
)

// FileSystem confines pull I/O to the configured artifact namespace. Every
// operation owns bounded directory handles; symlink checks are not followed by
// unrestricted pathname opens. No handles or directories are created by New.
// The configured root's existing ancestors are trusted configuration, not remote
// input. Arbitrary same-user writes cannot be prevented by an advisory lock;
// os.Root prevents such path swaps from redirecting our I/O outside the root.
type FileSystem struct {
	root string
	err  error
}

var _ ingest.FileSystem = (*FileSystem)(nil)

func NewFileSystem(root string) *FileSystem {
	abs, err := filepath.Abs(root)
	return &FileSystem{root: abs, err: err}
}

func (f *FileSystem) relative(path string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(f.root, abs)
	if err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("pull filesystem: path is outside the configured artifact root")
	}
	return rel, nil
}

func directoryChild(parent *os.Root, name string) (*os.Root, error) {
	info, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("pull filesystem: directory component is not a real directory")
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	opened, err := child.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		_ = child.Close()
		return nil, fmt.Errorf("pull filesystem: directory changed while opening bounded handle")
	}
	return child, nil
}

// createDirectory walks down from the first existing configured ancestor.
// Mkdir is atomic; an existing path is accepted only after bounded validation.
func createDirectory(path string) error {
	ancestor := filepath.Clean(path)
	for {
		_, err := os.Lstat(ancestor)
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return err
		}
		ancestor = parent
	}
	info, err := os.Lstat(ancestor)
	if err != nil {
		return err
	}
	if ancestor != path && info.Mode()&os.ModeSymlink != 0 {
		// A pre-existing configured ancestor (for example an XDG-home link) is
		// trusted configuration. Links within the artifact root are never trusted.
		info, err = os.Stat(ancestor)
		if err != nil {
			return err
		}
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("pull filesystem: configured ancestor is not a real directory")
	}
	root, err := os.OpenRoot(ancestor)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		return fmt.Errorf("pull filesystem: configured ancestor changed while opening")
	}
	rel, err := filepath.Rel(ancestor, path)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	for _, name := range strings.Split(rel, string(filepath.Separator)) {
		if err := root.Mkdir(name, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		child, err := directoryChild(root, name)
		if err != nil {
			return err
		}
		_ = root.Close()
		root = child
	}
	return nil
}

func (f *FileSystem) open() (*os.Root, error) {
	if f.err != nil {
		return nil, f.err
	}
	parent, err := os.OpenRoot(filepath.Dir(f.root))
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	return directoryChild(parent, filepath.Base(f.root))
}

func parentIn(root *os.Root, rel string) (*os.Root, string, error) {
	current, err := root.OpenRoot(".")
	if err != nil {
		return nil, "", err
	}
	parent := filepath.Dir(rel)
	if parent != "." {
		for _, name := range strings.Split(parent, string(filepath.Separator)) {
			child, err := directoryChild(current, name)
			_ = current.Close()
			if err != nil {
				return nil, "", err
			}
			current = child
		}
	}
	return current, filepath.Base(rel), nil
}

func (f *FileSystem) parent(path string) (*os.Root, string, error) {
	rel, err := f.relative(path)
	if err != nil {
		return nil, "", err
	}
	root, err := f.open()
	if err != nil {
		return nil, "", err
	}
	defer root.Close()
	return parentIn(root, rel)
}

func regularFile(parent *os.Root, name string, write bool, mode os.FileMode) (*os.File, error) {
	info, err := parent.Lstat(name)
	flags := os.O_RDONLY
	if write {
		flags = os.O_RDWR
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("pull filesystem: file component is not a regular file")
	}
	file, err := parent.OpenFile(name, flags, mode)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		_ = file.Close()
		return nil, fmt.Errorf("pull filesystem: file changed while opening bounded handle")
	}
	return file, nil
}

func (f *FileSystem) ReadFile(path string) ([]byte, error) {
	parent, name, err := f.parent(path)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	file, err := regularFile(parent, name, false, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}
func (f *FileSystem) WriteFile(path string, data []byte, mode os.FileMode) error {
	parent, name, err := f.parent(path)
	if err != nil {
		return err
	}
	defer parent.Close()
	// These are staged writes, not publication. Never truncate an existing inode:
	// even a regular file can be an unowned hard link. Unlink then exclusive-create
	// refuses a concurrent replacement rather than following or modifying it.
	if info, err := parent.Lstat(name); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("pull filesystem: staged destination is not a regular file")
		}
		if err := parent.Remove(name); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	file, err := parent.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	return errors.Join(err, file.Close())
}
func (f *FileSystem) MkdirAll(path string, mode os.FileMode) error {
	rel, err := f.relative(path)
	if err != nil {
		return err
	}
	if err := createDirectory(f.root); err != nil {
		return err
	}
	root, err := f.open()
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if rel == "." {
		return nil
	}
	for _, name := range strings.Split(rel, string(filepath.Separator)) {
		if err := root.Mkdir(name, mode); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		child, err := directoryChild(root, name)
		if err != nil {
			return err
		}
		_ = root.Close()
		root = child
	}
	return nil
}
func (f *FileSystem) Lstat(path string) (os.FileInfo, error) {
	parent, name, err := f.parent(path)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	return parent.Lstat(name)
}
func (f *FileSystem) Stat(path string) (os.FileInfo, error) {
	info, err := f.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("pull filesystem: symlink is not an artifact")
	}
	return info, nil
}
func (f *FileSystem) ReadDir(path string) ([]os.DirEntry, error) {
	parent, name, err := f.parent(path)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	root, err := directoryChild(parent, name)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.ReadDir(-1)
}
func (f *FileSystem) Rename(src, dst string) error {
	a, err := f.relative(src)
	if err != nil {
		return err
	}
	b, err := f.relative(dst)
	if err != nil {
		return err
	}
	root, err := f.open()
	if err != nil {
		return err
	}
	defer root.Close()
	parent, name, err := parentIn(root, a)
	if err != nil {
		return err
	}
	defer parent.Close()
	info, err := parent.Lstat(name)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("pull filesystem: refuse symlink publication")
	}
	target, leaf, err := parentIn(root, b)
	if err != nil {
		return err
	}
	defer target.Close()
	if info, err := target.Lstat(leaf); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("pull filesystem: refuse symlink replacement")
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return root.Rename(a, b)
}
func (f *FileSystem) Remove(path string) error {
	parent, name, err := f.parent(path)
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.Remove(name)
}
func (f *FileSystem) RemoveAll(path string) error {
	parent, name, err := f.parent(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.RemoveAll(name)
}
func (f *FileSystem) CopyFile(src, dst string, mode os.FileMode) error {
	data, err := f.ReadFile(src)
	if err != nil {
		return err
	}
	return f.WriteFile(dst, data, mode)
}
func (f *FileSystem) WalkDir(path string, fn fs.WalkDirFunc) error {
	rel, err := f.relative(path)
	if err != nil {
		return err
	}
	root, err := f.open()
	if err != nil {
		return err
	}
	defer root.Close()
	return fs.WalkDir(root.FS(), filepath.ToSlash(rel), func(name string, d fs.DirEntry, err error) error {
		return fn(filepath.Join(f.root, filepath.FromSlash(name)), d, err)
	})
}

// Sync also accepts ancestors of the configured root: persisting a newly created
// root entry requires flushing its existing parent, not merely the root itself.
func (f *FileSystem) Sync(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if rel, err := filepath.Rel(abs, f.root); err == nil && filepath.IsLocal(rel) && abs != f.root {
		return syncContentPath(abs)
	}
	parent, name, err := f.parent(path)
	if err != nil {
		return err
	}
	defer parent.Close()
	info, err := parent.Lstat(name)
	if err != nil {
		return err
	}
	if info.IsDir() {
		root, err := directoryChild(parent, name)
		if err != nil {
			return err
		}
		defer root.Close()
		if runtime.GOOS == "windows" {
			return nil
		}
		file, err := root.Open(".")
		if err != nil {
			return err
		}
		return errors.Join(file.Sync(), file.Close())
	}
	file, err := regularFile(parent, name, true, 0)
	if err != nil {
		return err
	}
	return errors.Join(file.Sync(), file.Close())
}
