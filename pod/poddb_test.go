package pod

import (
	"fmt"
	"testing"

	"gorm.io/gorm"
)

// mostly integration tests

type mockGorm struct {
	// todo: what do we need
	db *gorm.DB
}

func mockGormOpen() {
	fmt.Print("MockGorm.Open()")
	// tod
}

// func setupTest(t *testing.T, openDB bool, openError bool) (*mockgorm, func(*testing.T, *mockgorm)) {
	// var (
	// 	mock mockGorm
	// 	err  error
	// )
	// var oldGormOpen = gormOpen
	// gormOpen = mockGormOpen
	// fmt.Printf("SetupTest(%v)", t.Name())

	// if openDB {
	// 	fmt.Print(", opening db (inMemoryMode)")
	// 	mock.db, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &defaultConfig)
	// 	if err != nil {
	// 		t.Fatalf("create db failed: %v", err)
	// 	}

	// }
	// fmt.Print("\n")

	// return &mock, func(t *testing.T, m *mockClover) {
	// 	fmt.Printf("\nTeardown(%v)", t.Name())
	// 	if m.db != nil {
	// 		fmt.Print(", closing db")
	// 		m.db.Close()
	// 	}
	// 	fmt.Print("\n")
	// 	cimpl = oldclover
	// }
// }

func TestNewDB(t *testing.T) {
	type args struct {
	}
	type exp struct {
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// got, err := NewDB(tt.args.path)
			// if (err != nil) != tt.wantErr {
			// 	t.Errorf("NewDB() error = %v, wantErr %v", err, tt.wantErr)
			// 	return
			// }
			// if !reflect.DeepEqual(got, tt.want) {
			// 	t.Errorf("NewDB() = %v, want %v", got, tt.want)
			// }
		})
	}
}

/*
func TestPodDB_loadDBFeed(t *testing.T) {
	type args struct {
	}
	type exp struct {
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.pdb.loadDBFeed(tt.args.feedEntry, tt.args.loadXml); (err != nil) != tt.wantErr {
				t.Errorf("PodDB.loadDBFeed() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
*/ /*
func TestPodDB_loadDBFeedXml(t *testing.T) {
	type args struct {
	}
	type exp struct {
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.pdb.loadDBFeedXml(tt.args.feedXml); (err != nil) != tt.wantErr {
				t.Errorf("PodDB.loadDBFeedXml() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
*/ /*
func TestPodDB_loadFeedItems(t *testing.T) {
	type args struct {
	}
	type exp struct {
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.pdb.loadFeedItems(tt.args.feedId, tt.args.numItems, tt.args.includeXml)
			if (err != nil) != tt.wantErr {
				t.Errorf("PodDB.loadFeedItems() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PodDB.loadFeedItems() = %v, want %v", got, tt.want)
			}
		})
	}
}
*/ /*
func TestPodDB_saveFeed(t *testing.T) {
	type args struct {
	}
	type exp struct {
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.pdb.saveFeed(tt.args.feed); (err != nil) != tt.wantErr {
				t.Errorf("PodDB.saveFeed() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
*/
