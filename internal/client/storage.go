package client

import (
	"fmt"
	"net/netip"
	"sync"

	"github.com/AdguardTeam/golibs/errors"
)

// Storage contains information about persistent and runtime clients.
type Storage struct {
	// mu protects index of persistent clients.
	mu *sync.Mutex

	// index contains information about persistent clients.
	index *Index

	// runtimeIndex contains information about runtime clients.
	runtimeIndex map[netip.Addr]*Runtime
}

// NewStorage returns initialized client storage.
func NewStorage() (s *Storage) {
	return &Storage{
		mu:           &sync.Mutex{},
		index:        NewIndex(),
		runtimeIndex: map[netip.Addr]*Runtime{},
	}
}

// Add stores persistent client information or returns an error.  p must be
// valid persistent client.  See [Persistent.Validate].
func (s *Storage) Add(p *Persistent) (err error) {
	defer func() { err = errors.Annotate(err, "adding client: %w") }()

	s.mu.Lock()
	defer s.mu.Unlock()

	err = s.index.ClashesUID(p)
	if err != nil {
		// Don't wrap the error since there is already an annotation deferred.
		return err
	}

	err = s.index.Clashes(p)
	if err != nil {
		// Don't wrap the error since there is already an annotation deferred.
		return err
	}

	s.index.Add(p)

	return nil
}

// Find finds persistent client by string representation of the client ID, IP
// address, or MAC.  And returns it shallow copy.
func (s *Storage) Find(id string) (p *Persistent, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok = s.index.Find(id)
	if ok {
		return p.ShallowClone(), ok
	}

	return nil, false
}

// FindLoose is like [Storage.Find] but it also tries to find a persistent
// client by IP address without zone.  It strips the IPv6 zone index from the
// stored IP addresses before comparing, because querylog entries don't have it.
// See TODO on [querylog.logEntry.IP].
//
// Note that multiple clients can have the same IP address with different zones.
// Therefore, the result of this method is indeterminate.
func (s *Storage) FindLoose(ip netip.Addr, id string) (p *Persistent, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok = s.index.Find(id)
	if ok {
		return p.ShallowClone(), ok
	}

	p = s.index.FindByIPWithoutZone(ip)
	if p != nil {
		return p.ShallowClone(), true
	}

	return nil, false
}

// RemoveByName removes persistent client information.  ok is false if no such
// client exists by that name.
func (s *Storage) RemoveByName(name string) (ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.index.FindByName(name)
	if !ok {
		return false
	}

	s.index.Delete(p)

	return true
}

// Update finds the stored persistent client by its name and updates its
// information from n.
func (s *Storage) Update(name string, n *Persistent) (err error) {
	defer func() { err = errors.Annotate(err, "updating client: %w") }()

	s.mu.Lock()
	defer s.mu.Unlock()

	stored, ok := s.index.FindByName(name)
	if !ok {
		return fmt.Errorf("client %q is not found", name)
	}

	// Client n has a newly generated UID, so replace it with the stored one.
	//
	// TODO(s.chzhen):  Remove when frontend starts handling UIDs.
	n.UID = stored.UID

	err = s.index.Clashes(n)
	if err != nil {
		// Don't wrap the error since there is already an annotation deferred.
		return err
	}

	s.index.Delete(stored)
	s.index.Add(n)

	return nil
}

// RangeByName calls f for each persistent client sorted by name, unless cont is
// false.
func (s *Storage) RangeByName(f func(c *Persistent) (cont bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.index.RangeByName(f)
}

// CloseUpstreams closes upstream configurations of persistent clients.
func (s *Storage) CloseUpstreams() (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.index.CloseUpstreams()
}

// ClientRuntime returns the saved runtime client by ip.  If no such client
// exists, returns nil.
func (s *Storage) ClientRuntime(ip netip.Addr) (rc *Runtime) {
	return s.runtimeIndex[ip]
}

// AddRuntime saves the runtime client information in the storage.  IP address
// of a client must be unique.  rc must not be nil.
func (s *Storage) AddRuntime(rc *Runtime) {
	ip := rc.Addr()
	s.runtimeIndex[ip] = rc
}

// SizeRuntime returns the number of the runtime clients.
func (s *Storage) SizeRuntime() (n int) {
	return len(s.runtimeIndex)
}

// RangeRuntime calls f for each runtime client in an undefined order.
func (s *Storage) RangeRuntime(f func(rc *Runtime) (cont bool)) {
	for _, rc := range s.runtimeIndex {
		if !f(rc) {
			return
		}
	}
}

// DeleteRuntime removes the runtime client by ip.
func (s *Storage) DeleteRuntime(ip netip.Addr) {
	delete(s.runtimeIndex, ip)
}

// DeleteBySource removes all runtime clients that have information only from
// the specified source and returns the number of removed clients.
func (s *Storage) DeleteBySource(src Source) (n int) {
	for ip, rc := range s.runtimeIndex {
		rc.unset(src)

		if rc.isEmpty() {
			delete(s.runtimeIndex, ip)
			n++
		}
	}

	return n
}
