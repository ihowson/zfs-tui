package main

import (
	"sort"

	"github.com/mistifyio/go-zfs/v4"
)

type snapshotItem struct {
	Name string
	Used uint64
}

func listSnapshots(fsName string) ([]snapshotItem, error) {
	datasets, err := zfs.Snapshots(fsName)
	if err != nil {
		return nil, err
	}
	items := make([]snapshotItem, len(datasets))
	for i, ds := range datasets {
		items[i] = snapshotItem{Name: ds.Name, Used: ds.Used}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func destroySnapshot(name string) error {
	ds, err := zfs.GetDataset(name)
	if err != nil {
		return err
	}
	return ds.Destroy(zfs.DestroyDefault)
}