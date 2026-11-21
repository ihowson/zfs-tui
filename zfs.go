package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mistifyio/go-zfs/v4"
)

// datasetItem is one entry in the dataset browser.
type datasetItem struct {
	Name string
	Used uint64
	// up marks the ".." row that navigates to the parent dataset.
	up bool
}

// snapshotItem is one snapshot of the dataset currently being browsed.
type snapshotItem struct {
	Name  string // full name: dataset@snapshot
	Short string // snapshot part only
	Used  uint64
	Refer uint64
}

// checkFilesystem verifies that name is an existing ZFS filesystem.
func checkFilesystem(name string) error {
	if strings.ContainsRune(name, '@') {
		return fmt.Errorf("%q is a snapshot, not a filesystem", name)
	}
	ds, err := zfs.GetDataset(name)
	if err != nil {
		return fmt.Errorf("cannot open dataset %q: %w", name, err)
	}
	if ds.Type != zfs.DatasetFilesystem {
		return fmt.Errorf("%q is a ZFS %s, not a filesystem", name, ds.Type)
	}
	return nil
}

// listPools returns the root datasets (pools) as browser entries, sorted by name.
func listPools() ([]datasetItem, error) {
	all, err := zfs.Filesystems("")
	if err != nil {
		return nil, err
	}
	items := make([]datasetItem, 0, len(all))
	for _, ds := range all {
		if strings.ContainsAny(ds.Name, "/@") {
			continue
		}
		items = append(items, datasetItem{Name: ds.Name, Used: ds.Used})
	}
	slices.SortFunc(items, func(a, b datasetItem) int {
		return strings.Compare(a.Name, b.Name)
	})
	return items, nil
}

// listChildren returns the immediate child datasets of parent, sorted by name.
func listChildren(parent string) ([]datasetItem, error) {
	all, err := zfs.Datasets(parent)
	if err != nil {
		return nil, err
	}
	items := make([]datasetItem, 0, len(all))
	for _, ds := range all {
		rest, ok := strings.CutPrefix(ds.Name, parent+"/")
		if !ok || strings.ContainsAny(rest, "/@") {
			continue
		}
		items = append(items, datasetItem{Name: ds.Name, Used: ds.Used})
	}
	slices.SortFunc(items, func(a, b datasetItem) int {
		return strings.Compare(a.Name, b.Name)
	})
	return items, nil
}

// listSnapshots returns the snapshots of fsName, sorted by name.
func listSnapshots(fsName string) ([]snapshotItem, error) {
	datasets, err := zfs.Snapshots(fsName)
	if err != nil {
		return nil, err
	}
	items := make([]snapshotItem, 0, len(datasets))
	for _, ds := range datasets {
		short := ds.Name
		if _, s, ok := strings.Cut(ds.Name, "@"); ok {
			short = s
		}
		items = append(items, snapshotItem{
			Name:  ds.Name,
			Short: short,
			Used:  ds.Used,
			Refer: ds.Referenced,
		})
	}
	slices.SortFunc(items, func(a, b snapshotItem) int {
		return strings.Compare(a.Name, b.Name)
	})
	return items, nil
}

// poolAvail returns the free space of the pool containing fsName.
func poolAvail(fsName string) (uint64, error) {
	ds, err := zfs.GetDataset(poolName(fsName))
	if err != nil {
		return 0, err
	}
	return ds.Avail, nil
}

func poolName(fsName string) string {
	pool, _, _ := strings.Cut(fsName, "/")
	return pool
}

// destroySnapshot destroys a single snapshot by full name.
func destroySnapshot(name string) error {
	ds, err := zfs.GetDataset(name)
	if err != nil {
		return err
	}
	return ds.Destroy(zfs.DestroyDefault)
}
