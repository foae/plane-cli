package config

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Store holds an open directory descriptor so later path replacement cannot
// redirect writes. All children are simple opaque names, never profile paths.
type Store struct{ dir *os.File }

func Directory() (string, error) {
	if s := os.Getenv("PLANE_CLI_CONFIG_DIR"); s != "" {
		return s, nil
	}
	s, e := os.UserConfigDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(s, "foae-plane-cli"), nil
}

func Open(path string, create bool) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.TrimPrefix(filepath.Clean(abs), "/"), "/")
	for i, part := range parts {
		next, e := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if errors.Is(e, unix.ENOENT) && create {
			if e = unix.Mkdirat(fd, part, 0700); e == nil || errors.Is(e, unix.EEXIST) {
				next, e = unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			}
		}
		if e != nil {
			_ = unix.Close(fd)
			return nil, fmt.Errorf("cannot open safe configuration directory: %w", e)
		}
		var st unix.Stat_t
		e = unix.Fstat(next, &st)
		// Root-owned sticky temporary parents are permitted; the final directory
		// must belong to this user and be inaccessible to other users.
		unsafe := e != nil || (st.Uid != 0 && st.Uid != uint32(os.Geteuid())) || (st.Mode&0022 != 0 && st.Mode&unix.S_ISVTX == 0)
		if i == len(parts)-1 {
			unsafe = unsafe || st.Uid != uint32(os.Geteuid()) || st.Mode&0077 != 0
		}
		_ = unix.Close(fd)
		if unsafe {
			_ = unix.Close(next)
			return nil, fmt.Errorf("configuration directory %q has unsafe ownership or permissions (uid=%d mode=%o)", filepath.Join(append([]string{"/"}, parts[:i+1]...)...), st.Uid, st.Mode&07777)
		}
		fd = next
	}
	return &Store{dir: os.NewFile(uintptr(fd), abs)}, nil
}

func (s *Store) Close() error { return s.dir.Close() }
func child(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\\x00")
}

func (s *Store) Read(name string) ([]byte, error) {
	if !child(name) {
		return nil, errors.New("invalid storage name")
	}
	fd, e := unix.Openat(int(s.dir.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), name)
	defer func() { _ = f.Close() }()
	if e = secureFile(f); e != nil {
		return nil, e
	}
	b, e := io.ReadAll(io.LimitReader(f, 8<<20+1))
	if e != nil {
		return nil, e
	}
	if len(b) > 8<<20 {
		return nil, errors.New("local record exceeds 8 MiB")
	}
	return b, nil
}
func secureFile(f *os.File) error {
	var st unix.Stat_t
	if e := unix.Fstat(int(f.Fd()), &st); e != nil {
		return e
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG || st.Uid != uint32(os.Geteuid()) || st.Mode&0077 != 0 || st.Nlink != 1 {
		return errors.New("local record ownership or permissions are unsafe")
	}
	return nil
}
func (s *Store) Write(name string, data []byte) error {
	if !child(name) {
		return errors.New("invalid storage name")
	}
	if len(data) > 8<<20 {
		return errors.New("local record exceeds 8 MiB")
	}
	// Refuse an existing unsafe destination, even though rename would replace it.
	if _, e := s.Read(name); e != nil && !errors.Is(e, os.ErrNotExist) {
		return e
	}
	suffix := make([]byte, 16)
	if _, e := rand.Read(suffix); e != nil {
		return e
	}
	temp := ".tmp-" + hex.EncodeToString(suffix)
	fd, e := unix.Openat(int(s.dir.Fd()), temp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if e != nil {
		return e
	}
	f := os.NewFile(uintptr(fd), temp)
	defer func() { _ = f.Close(); _ = unix.Unlinkat(int(s.dir.Fd()), temp, 0) }()
	if _, e = f.Write(data); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = unix.Renameat(int(s.dir.Fd()), temp, int(s.dir.Fd()), name); e != nil {
		return e
	}
	return s.dir.Sync()
}
func (s *Store) Remove(name string) error {
	if !child(name) {
		return errors.New("invalid storage name")
	}
	if _, e := s.Read(name); errors.Is(e, os.ErrNotExist) {
		return nil
	} else if e != nil {
		return e
	}
	if e := unix.Unlinkat(int(s.dir.Fd()), name, 0); e != nil {
		return e
	}
	return s.dir.Sync()
}

// Lock uses a stable inode, never the atomically replaced data file. Callers
// needing both locks acquire ConnectionLock before the configuration lock.
func (s *Store) Lock(ctx context.Context, name string) (func() error, error) {
	if !child(name) {
		return nil, errors.New("invalid lock name")
	}
	fd, e := unix.Openat(int(s.dir.Fd()), name, unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0600)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), name)
	if e = secureFile(f); e != nil {
		_ = f.Close()
		return nil, e
	}
	for {
		if e = ctx.Err(); e != nil {
			_ = f.Close()
			return nil, e
		}
		e = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
		if e == nil {
			return f.Close, nil
		}
		if !errors.Is(e, unix.EWOULDBLOCK) && !errors.Is(e, unix.EAGAIN) {
			_ = f.Close()
			return nil, e
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = f.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
func (s *Store) ConnectionLock(ctx context.Context, name string) (func() error, error) {
	sum := sha256.Sum256([]byte(name))
	return s.Lock(ctx, "connection-"+hex.EncodeToString(sum[:])+".lock")
}
func (s *Store) Load() (File, error) {
	f := File{Version: Version, Connections: map[string]Connection{}}
	b, e := s.Read("config.json")
	if errors.Is(e, os.ErrNotExist) {
		return f, nil
	}
	if e != nil {
		return f, e
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&f); e != nil {
		return f, errors.New("configuration JSON is invalid")
	}
	if e = decoder.Decode(new(any)); !errors.Is(e, io.EOF) {
		return f, errors.New("configuration JSON is invalid")
	}
	if f.Version != Version {
		return f, errors.New("unsupported configuration version; file left unchanged")
	}
	if f.Connections == nil {
		f.Connections = map[string]Connection{}
	}
	return f, nil
}
func (s *Store) Update(ctx context.Context, change func(*File) error) error {
	unlock, e := s.Lock(ctx, "config.lock")
	if e != nil {
		return e
	}
	defer func() { _ = unlock() }()
	f, e := s.Load()
	if e != nil {
		return e
	}
	if e = change(&f); e != nil {
		return e
	}
	b, e := json.Marshal(f)
	if e != nil {
		return e
	}
	return s.Write("config.json", append(b, '\n'))
}

func NewIdentity() (string, error) {
	b := make([]byte, 16)
	_, e := rand.Read(b)
	return hex.EncodeToString(b), e
}
