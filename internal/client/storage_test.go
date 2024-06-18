package client_test

import (
	"net/netip"
	"testing"

	"github.com/AdguardTeam/AdGuardHome/internal/client"
	"github.com/AdguardTeam/golibs/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorage_Add(t *testing.T) {
	const (
		existingName     = "existing_name"
		existingClientID = "existing_client_id"
	)

	var (
		existingClientUID = client.MustNewUID()
		existingIP        = netip.MustParseAddr("1.2.3.4")
		existingSubnet    = netip.MustParsePrefix("1.2.3.0/24")
	)

	existingClient := &client.Persistent{
		Name:      existingName,
		IPs:       []netip.Addr{existingIP},
		Subnets:   []netip.Prefix{existingSubnet},
		ClientIDs: []string{existingClientID},
		UID:       existingClientUID,
	}

	s := client.NewStorage()
	err := s.Add(existingClient)
	require.NoError(t, err)

	testCases := []struct {
		name       string
		cli        *client.Persistent
		wantErrMsg string
	}{{
		name: "basic",
		cli: &client.Persistent{
			Name: "basic",
			IPs:  []netip.Addr{netip.MustParseAddr("1.1.1.1")},
			UID:  client.MustNewUID(),
		},
		wantErrMsg: "",
	}, {
		name: "duplicate_uid",
		cli: &client.Persistent{
			Name: "no_uid",
			IPs:  []netip.Addr{netip.MustParseAddr("2.2.2.2")},
			UID:  existingClientUID,
		},
		wantErrMsg: `adding client: another client "existing_name" uses the same uid`,
	}, {
		name: "duplicate_name",
		cli: &client.Persistent{
			Name: existingName,
			IPs:  []netip.Addr{netip.MustParseAddr("3.3.3.3")},
			UID:  client.MustNewUID(),
		},
		wantErrMsg: `adding client: another client uses the same name "existing_name"`,
	}, {
		name: "duplicate_ip",
		cli: &client.Persistent{
			Name: "duplicate_ip",
			IPs:  []netip.Addr{existingIP},
			UID:  client.MustNewUID(),
		},
		wantErrMsg: `adding client: another client "existing_name" uses the same IP "1.2.3.4"`,
	}, {
		name: "duplicate_subnet",
		cli: &client.Persistent{
			Name:    "duplicate_subnet",
			Subnets: []netip.Prefix{existingSubnet},
			UID:     client.MustNewUID(),
		},
		wantErrMsg: `adding client: another client "existing_name" ` +
			`uses the same subnet "1.2.3.0/24"`,
	}, {
		name: "duplicate_client_id",
		cli: &client.Persistent{
			Name:      "duplicate_client_id",
			ClientIDs: []string{existingClientID},
			UID:       client.MustNewUID(),
		},
		wantErrMsg: `adding client: another client "existing_name" ` +
			`uses the same ClientID "existing_client_id"`,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err = s.Add(tc.cli)

			testutil.AssertErrorMsg(t, tc.wantErrMsg, err)
		})
	}
}

func TestStorage_RemoveByName(t *testing.T) {
	const (
		existingName = "existing_name"
	)

	existingClient := &client.Persistent{
		Name: existingName,
		IPs:  []netip.Addr{netip.MustParseAddr("1.2.3.4")},
		UID:  client.MustNewUID(),
	}

	s := client.NewStorage()
	err := s.Add(existingClient)
	require.NoError(t, err)

	testCases := []struct {
		want    assert.BoolAssertionFunc
		name    string
		cliName string
	}{{
		name:    "existing_client",
		cliName: existingName,
		want:    assert.True,
	}, {
		name:    "non_existing_client",
		cliName: "non_existing_client",
		want:    assert.False,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.want(t, s.RemoveByName(tc.cliName))
		})
	}

	t.Run("duplicate_remove", func(t *testing.T) {
		s = client.NewStorage()
		err = s.Add(existingClient)
		require.NoError(t, err)

		assert.True(t, s.RemoveByName(existingName))
		assert.False(t, s.RemoveByName(existingName))
	})
}
