package v8

import (
	"time"

	"github.com/giantswarm/versionbundle"
)

func VersionBundle() versionbundle.Bundle {
	return versionbundle.Bundle{
		Changelogs: []versionbundle.Changelog{
			{
				Component:   "cloudconfig",
				Description: "Put your description here",
				Kind:        versionbundle.KindChanged,
			},
		},
		Components: []versionbundle.Component{
			{
				Name:    "calico",
				Version: "3.0.2",
			},
			{
				Name:    "containerlinux",
				Version: "1576.5.0",
			},
			{
				Name:    "docker",
				Version: "17.09.0",
			},
			{
				Name:    "etcd",
				Version: "3.3.1",
			},
			{
				Name:    "coredns",
				Version: "1.0.6",
			},
			{
				Name:    "kubernetes",
				Version: "1.9.2",
			},
			{
				Name:    "nginx-ingress-controller",
				Version: "0.11.0",
			},
		},
		Dependencies: []versionbundle.Dependency{},
		Deprecated:   false,
		Name:         "kvm-operator",
		Time:         time.Date(2018, time.March, 7, 2, 57, 0, 0, time.UTC),
		Version:      "2.1.1",
		WIP:          true,
	}
}