package main

import (
	"sort"
	"strings"

	"github.com/mistifyio/go-zfs/v4"
)

type snapshotItem struct {
	Name  string
	Used  uint64
	Refer uint64
}

func listSnapshots(fsName string) ([]snapshotItem, error) {
	datasets, err := zfs.Snapshots(fsName)
	if err != nil {
		return nil, err
	}
	items := make([]snapshotItem, len(datasets))
	for i, ds := range datasets {
		items[i] = snapshotItem{Name: ds.Name, Used: ds.Used, Refer: ds.Referenced}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func poolAvail(fsName string) (uint64, error) {
	ds, err := zfs.GetDataset(poolName(fsName))
	if err != nil {
		return 0, err
	}
	return ds.Avail, nil
}

func poolName(fsName string) string {
	idx := strings.IndexByte(fsName, '/')
	if idx < 0 {
		return fsName
	}
	return fsName[:idx]
}

func destroySnapshot(name string) error {
	ds, err := zfs.GetDataset(name)
	if err != nil {
		return err
	}
	return ds.Destroy(zfs.DestroyDefault)
}