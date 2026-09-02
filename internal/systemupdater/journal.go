package systemupdater

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	contract "github.com/QuantumNous/new-api/pkg/systemupdate"
)

type Journal struct {
	dir string
	mu  sync.Mutex
}

func NewJournal(stateDir string) *Journal {
	return &Journal{dir: filepath.Join(stateDir, "operations")}
}

func (j *Journal) Save(operation contract.Operation) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if err := os.MkdirAll(j.dir, 0700); err != nil {
		return fmt.Errorf("create operation journal: %w", err)
	}
	data, err := common.Marshal(operation)
	if err != nil {
		return fmt.Errorf("encode operation journal: %w", err)
	}
	path := filepath.Join(j.dir, operation.ID+".json")
	temporary, err := os.CreateTemp(j.dir, ".operation-*.tmp")
	if err != nil {
		return fmt.Errorf("create operation journal temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return fmt.Errorf("secure operation journal: %w", err)
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		temporary.Close()
		return fmt.Errorf("write operation journal: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync operation journal: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close operation journal: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("commit operation journal: %w", err)
	}
	return nil
}

func (j *Journal) Get(operationID string) (contract.Operation, error) {
	if !contract.ValidIdempotencyKey(operationID) || strings.ContainsAny(operationID, `/\\\x00`) {
		return contract.Operation{}, os.ErrNotExist
	}
	data, err := os.ReadFile(filepath.Join(j.dir, operationID+".json"))
	if err != nil {
		return contract.Operation{}, err
	}
	var operation contract.Operation
	if err := common.Unmarshal(data, &operation); err != nil {
		return contract.Operation{}, fmt.Errorf("decode operation journal: %w", err)
	}
	return operation, nil
}

func (j *Journal) List() ([]contract.Operation, error) {
	entries, err := os.ReadDir(j.dir)
	if os.IsNotExist(err) {
		return []contract.Operation{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list operation journal: %w", err)
	}
	operations := make([]contract.Operation, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		operation, err := j.Get(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		operations = append(operations, operation)
	}
	sort.Slice(operations, func(i, k int) bool {
		if operations[i].CreatedAt == operations[k].CreatedAt {
			return operations[i].ID > operations[k].ID
		}
		return operations[i].CreatedAt > operations[k].CreatedAt
	})
	return operations, nil
}
