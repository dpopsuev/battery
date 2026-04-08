package testkit_test

import (
	"testing"

	"github.com/dpopsuev/battery/testkit"
)

func TestEventLogContract_StubEventLog(t *testing.T) {
	log := testkit.NewStubEventLog()
	testkit.RunEventLogContract(t, log)
}
