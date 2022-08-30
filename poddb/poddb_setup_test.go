package poddb

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ostafen/clover/v2"
)

// common functions used for all tests

type mockClover struct {
	db        *clover.DB
	openError bool
}

func (mc mockClover) Open(p string, _ ...clover.Option) (*clover.DB, error) {
	fmt.Print("MockClover.Open()")
	var err error
	// db might have been opened by setupTest()
	if mc.openError {
		fmt.Println(", returning open error")
		return nil, errors.New("foobar")
	} else if mc.db == nil {
		fmt.Println(", opening db (inMemoryMode)")
		mc.db, err = clover.Open(p, clover.InMemoryMode(true))
	} else {
		fmt.Println(", reusing already open connection")
	}
	return mc.db, err
}

func (mc mockClover) Close() error {
	fmt.Println("MockClover.Close(), faking closing db")
	return nil // need to explicitly close in teardown function
}

func (mc mockClover) FinalClose(t *testing.T) {

}

func setupTest(t *testing.T, openDB bool, openError bool) (*mockClover, func(*testing.T, *mockClover)) {
	var (
		mock = mockClover{openError: openError}
		err  error
	)
	var oldclover = cimpl
	cimpl = mock
	fmt.Print("SetupTest()")

	if openDB {
		fmt.Print(", opening db (inMemoryMode)")
		mock.db, err = clover.Open(t.Name(), clover.InMemoryMode(true))
		if err != nil {
			t.Fatalf("create db failed: %v", err)
		}
	}
	fmt.Print("\n")

	return &mock, func(t *testing.T, m *mockClover) {
		fmt.Print("Teardown()")
		if m.db != nil {
			fmt.Print(", closing db")
			m.db.Close()
		}
		fmt.Print("\n")
		cimpl = oldclover
	}
}
