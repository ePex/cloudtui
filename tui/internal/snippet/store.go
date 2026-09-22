package snippet

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrExists is returned by Save, MkDir, and Move when something of that
// name already exists (and, for Save, overwrite is false).
var ErrExists = errors.New("already exists")

// errUnavailable is returned by every Store operation when the snippets
// root couldn't be resolved (see DefaultRoot).
var errUnavailable = errors.New("snippets folder unavailable")

// Store reads and writes snippet files below a root folder. The folder
// tree on disk is the library's structure; there is no index file.
type Store struct {
	root string
}

// Entry is one item in a snippets folder: a subfolder or a snippet file.
// IsLink is set for a symlink (IsDir then describes what it points to).
type Entry struct {
	Name   string
	IsDir  bool
	IsLink bool
}

// NewStore returns a Store rooted at root. An empty root yields a Store
// whose operations all fail with a "snippets folder unavailable" error.
func NewStore(root string) *Store {
	return &Store{root: root}
}

// DefaultRoot returns ~/.cloudtui/snippets, next to config.yaml.
func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".cloudtui", "snippets"), nil
}

// Root returns the store's root folder ("" if unavailable).
func (s *Store) Root() string { return s.root }

// List returns the entries of dir (relative to the root; "" is the root
// itself): folders first, then snippets, each sorted case-insensitively.
// Hidden entries (names starting with ".") and anything that is neither
// a folder nor a regular file are skipped; symlinks are classified by
// their target. A root that doesn't exist yet lists as empty.
func (s *Store) List(dir string) ([]Entry, error) {
	path, err := s.resolve(dir, true)
	if err != nil {
		return nil, err
	}
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		if dir == "" && errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing snippets: %w", err)
	}

	entries := make([]Entry, 0, len(dirEntries))
	for _, de := range dirEntries {
		name := de.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		e, ok := classify(filepath.Join(path, name))
		if !ok {
			// A dangling symlink, something that's neither a folder nor a
			// regular file, or an entry removed since ReadDir.
			continue
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		la, lb := strings.ToLower(a.Name), strings.ToLower(b.Name)
		if la != lb {
			return la < lb
		}
		return a.Name < b.Name
	})
	return entries, nil
}

// Load reads and parses the snippet at name (relative to the root).
func (s *Store) Load(name string) (Snippet, error) {
	path, err := s.resolve(name, false)
	if err != nil {
		return Snippet{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Snippet{}, fmt.Errorf("reading snippet %q: %w", name, err)
	}
	sn, err := Parse(data)
	if err != nil {
		return Snippet{}, fmt.Errorf("snippet %q: %w", name, err)
	}
	return sn, nil
}

// Save writes sn to name (relative to the root; see ValidateName),
// creating missing parent folders. Without overwrite, an existing file
// is left untouched and ErrExists is returned — the existence check and
// the create are a single O_EXCL open, so there's no race between them.
func (s *Store) Save(name string, sn Snippet, overwrite bool) error {
	if s.root == "" {
		return errUnavailable
	}
	rel, err := ValidateName(name)
	if err != nil {
		return err
	}
	sn.Body = FormatBody(sn.Body)
	path := filepath.Join(s.root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating snippet folder: %w", err)
	}

	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if !overwrite {
		flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return ErrExists
		}
		return fmt.Errorf("saving snippet %q: %w", name, err)
	}
	if _, err := f.Write(Format(sn)); err != nil {
		f.Close()
		return fmt.Errorf("saving snippet %q: %w", name, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("saving snippet %q: %w", name, err)
	}
	return nil
}

// ValidateName checks a user-entered snippet name and returns it as a
// root-relative path with OS separators. "/" separates subfolders on
// every OS. Empty names, a trailing separator, absolute paths, ".."
// segments, reserved names, and hidden segments (which List would never
// show) are rejected.
func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("snippet name is required")
	}
	if strings.HasSuffix(name, "/") || strings.HasSuffix(name, string(filepath.Separator)) {
		return "", fmt.Errorf("snippet name %q must not end with a folder separator", name)
	}
	rel := filepath.FromSlash(name)
	if !filepath.IsLocal(rel) {
		return "", fmt.Errorf("snippet name %q must stay inside the snippets folder", name)
	}
	for _, seg := range strings.Split(filepath.ToSlash(filepath.Clean(rel)), "/") {
		if strings.HasPrefix(seg, ".") {
			return "", fmt.Errorf("snippet name %q must not contain hidden (\".\"-prefixed) parts", name)
		}
	}
	return filepath.Clean(rel), nil
}

// MkDir creates the folder name (relative to the root; see ValidateName)
// and any missing parents. Something already existing at name — folder
// or file — is an error.
func (s *Store) MkDir(name string) error {
	if s.root == "" {
		return errUnavailable
	}
	rel, err := ValidateName(name)
	if err != nil {
		return err
	}
	path := filepath.Join(s.root, rel)
	if err := notExisting(path, name); err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("creating folder %q: %w", name, err)
	}
	return nil
}

// Move renames or moves the snippet or folder from to to (both relative
// to the root; to must pass ValidateName). Missing parent folders of to
// are created. Nothing is overwritten: an existing to fails with
// ErrExists. Moving a folder into itself or one of its own subfolders is
// refused. Moving across filesystems (e.g. into a symlinked folder on
// another volume) fails with the OS error.
func (s *Store) Move(from, to string) error {
	src, err := s.resolve(from, false)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(src); err != nil {
		return fmt.Errorf("moving %q: %w", from, err)
	}
	rel, err := ValidateName(to)
	if err != nil {
		return err
	}
	fromRel := filepath.Clean(filepath.FromSlash(from))
	if rel == fromRel || strings.HasPrefix(rel, fromRel+string(filepath.Separator)) {
		return fmt.Errorf("can't move %q into itself", from)
	}
	dst := filepath.Join(s.root, rel)
	if err := notExisting(dst, to); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("creating folder for %q: %w", to, err)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("moving %q to %q: %w", from, to, err)
	}
	return nil
}

// Delete removes the snippet file name (relative to the root). Folders
// are refused; see DeleteFolder.
func (s *Store) Delete(name string) error {
	path, err := s.resolve(name, false)
	if err != nil {
		return err
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return fmt.Errorf("%q is a folder, not a snippet", name)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("deleting snippet %q: %w", name, err)
	}
	return nil
}

// DeleteFolder removes the folder dir (relative to the root) and
// everything in it. The root itself is refused. A symlinked folder has
// only its link removed — never the folder it points to — so a linked
// shared checkout is never deleted from here.
func (s *Store) DeleteFolder(dir string) error {
	path, err := s.resolve(dir, false)
	if err != nil {
		return err
	}
	e, ok := classify(path)
	if !ok || !e.IsDir {
		return fmt.Errorf("%q is not a folder", dir)
	}
	if e.IsLink {
		err = os.Remove(path)
	} else {
		err = os.RemoveAll(path)
	}
	if err != nil {
		return fmt.Errorf("deleting folder %q: %w", dir, err)
	}
	return nil
}

// Count returns how many snippets and folders are nested anywhere inside
// dir (relative to the root; "" is the root), by List's rules: hidden
// entries and broken links are skipped. A symlinked folder counts as one
// folder but isn't looked inside, since deleting removes only the link.
func (s *Store) Count(dir string) (snippets, folders int, err error) {
	path, err := s.resolve(dir, true)
	if err != nil {
		return 0, 0, err
	}
	var walk func(string) error
	walk = func(p string) error {
		des, err := os.ReadDir(p)
		if err != nil {
			return err
		}
		for _, de := range des {
			if strings.HasPrefix(de.Name(), ".") {
				continue
			}
			e, ok := classify(filepath.Join(p, de.Name()))
			switch {
			case !ok:
			case !e.IsDir:
				snippets++
			default:
				folders++
				if !e.IsLink {
					if err := walk(filepath.Join(p, de.Name())); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	if err := walk(path); err != nil {
		return 0, 0, fmt.Errorf("counting snippets in %q: %w", dir, err)
	}
	return snippets, folders, nil
}

// Stat describes the snippet or folder name (relative to the root).
func (s *Store) Stat(name string) (Entry, error) {
	path, err := s.resolve(name, false)
	if err != nil {
		return Entry{}, err
	}
	e, ok := classify(path)
	if !ok {
		return Entry{}, fmt.Errorf("%q is not a snippet or folder: %w", name, fs.ErrNotExist)
	}
	return e, nil
}

// classify describes path as an Entry, following symlinks to tell a
// folder from a file. ok is false for anything List would skip: a broken
// link, something that's neither a folder nor a regular file, or a path
// that doesn't exist.
func classify(path string) (e Entry, ok bool) {
	linfo, err := os.Lstat(path)
	if err != nil {
		return Entry{}, false
	}
	info, err := os.Stat(path)
	if err != nil {
		return Entry{}, false
	}
	e = Entry{Name: filepath.Base(path), IsLink: linfo.Mode()&fs.ModeSymlink != 0}
	switch {
	case info.IsDir():
		e.IsDir = true
	case !info.Mode().IsRegular():
		return Entry{}, false
	}
	return e, true
}

// notExisting returns ErrExists (naming name) when something — even a
// broken link — is already at path.
func notExisting(path, name string) error {
	_, err := os.Lstat(path)
	switch {
	case err == nil:
		return fmt.Errorf("%q: %w", name, ErrExists)
	case errors.Is(err, fs.ErrNotExist):
		return nil
	default:
		return fmt.Errorf("checking %q: %w", name, err)
	}
}

// resolve turns a root-relative path from the picker into an absolute
// one, refusing anything outside the root. allowRoot permits "" (the
// root itself), which only List accepts.
func (s *Store) resolve(rel string, allowRoot bool) (string, error) {
	if s.root == "" {
		return "", errUnavailable
	}
	if rel == "" && allowRoot {
		return s.root, nil
	}
	rel = filepath.FromSlash(rel)
	if !filepath.IsLocal(rel) {
		return "", fmt.Errorf("snippet path %q is outside the snippets folder", rel)
	}
	return filepath.Join(s.root, rel), nil
}
