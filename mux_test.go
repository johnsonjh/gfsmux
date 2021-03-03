package gfsmux_test

import (
	"bytes"
	"testing"

	smux "github.com/johnsonjh/gfsmux"
	u "github.com/johnsonjh/leaktestfe"
)

type buffer struct {
	bytes.Buffer
}

func (
	b *buffer,
) Close() error {
	b.Buffer.Reset()
	return nil
}

func TestConfig(
	t *testing.T,
) {
	defer u.Leakplug(
		t,
	)
	smux.VerifyConfig(
		smux.DefaultConfig(),
	)

	Config := smux.DefaultConfig()
	Config.KeepAliveInterval = 0
	err := smux.VerifyConfig(
		Config,
	)
	t.Log(
		err,
	)
	if err == nil {
		t.Fatal(
			err,
		)
	}

	Config = smux.DefaultConfig()
	Config.KeepAliveInterval = 10
	Config.KeepAliveTimeout = 5
	err = smux.VerifyConfig(
		Config,
	)
	t.Log(
		err,
	)
	if err == nil {
		t.Fatal(
			err,
		)
	}

	Config = smux.DefaultConfig()
	Config.MaxFrameSize = 0
	err = smux.VerifyConfig(
		Config,
	)
	t.Log(
		err,
	)
	if err == nil {
		t.Fatal(
			err,
		)
	}

	Config = smux.DefaultConfig()
	Config.MaxFrameSize = 65536
	err = smux.VerifyConfig(
		Config,
	)
	t.Log(
		err,
	)
	if err == nil {
		t.Fatal(
			err,
		)
	}

	Config = smux.DefaultConfig()
	Config.MaxReceiveBuffer = 0
	err = smux.VerifyConfig(
		Config,
	)
	t.Log(
		err,
	)
	if err == nil {
		t.Fatal(
			err,
		)
	}

	Config = smux.DefaultConfig()
	Config.MaxStreamBuffer = 0
	err = smux.VerifyConfig(
		Config,
	)
	t.Log(
		err,
	)
	if err == nil {
		t.Fatal(
			err,
		)
	}

	Config = smux.DefaultConfig()
	Config.MaxStreamBuffer = 100
	Config.MaxReceiveBuffer = 99
	err = smux.VerifyConfig(
		Config,
	)
	t.Log(
		err,
	)
	if err == nil {
		t.Fatal(
			err,
		)
	}

	var bts buffer
	if _, err := smux.Server(
		&bts,
		Config,
	); err == nil {
		t.Fatal(
			"server started with wrong Config",
		)
	}

	if _, err := smux.Client(
		&bts,
		Config,
	); err == nil {
		t.Fatal(
			"client started with wrong Config",
		)
	}
}
