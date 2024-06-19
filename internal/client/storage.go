package client

import (
	"fmt"
	"net"
	"net/netip"
	"sync"

	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/log"
)

// Storage contains information about persistent and runtime clients.
type Storage struct {
	// mu protects index of persistent clients.
	mu *sync.Mutex

	// index contains information about persistent clients.
	index *Index
}

// NewStorage returns initialized client storage.
func NewStorage() (s *Storage) {
	return &Storage{
		mu:    &sync.Mutex{},
		index: NewIndex(),
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

	log.Debug("client storage: added %q: IDs: %q [%d]", p.Name, p.IDs(), s.index.Size())

	return nil
}

// FindByName finds persistent client by name.
func (s *Storage) FindByName(name string) (c *Persistent, found bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.index.FindByName(name)
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

// FindByMAC finds persistent client by MAC.
func (s *Storage) FindByMAC(mac net.HardwareAddr) (c *Persistent, found bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.index.FindByMAC(mac)
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

	if err := p.CloseUpstreams(); err != nil {
		log.Error("client storage: removing client %q: %s", p.Name, err)
	}

	s.index.Delete(p)

	return true
}

// Update finds the stored persistent client by its name and updates its
// information from n.  n must be valid persistent client.  See
// [Persistent.Validate].
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

// Size returns the number of persistent clients.
func (s *Storage) Size() (n int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.index.Size()
}

// CloseUpstreams closes upstream configurations of persistent clients.
func (s *Storage) CloseUpstreams() (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.index.CloseUpstreams()
}
