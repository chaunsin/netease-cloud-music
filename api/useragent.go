// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import "sync"

type UserAgent struct {
	Android []string `json:"android" yaml:"android"`
	IOS     []string `json:"ios" yaml:"ios"`
	Mac     []string `json:"mac" yaml:"mac"`
	Windows []string `json:"windows" yaml:"windows"`
	Linux   []string `json:"linux" yaml:"linux"`
}

func (u *UserAgent) Get(os string) string {
	switch os {
	case "android":
		return u.Android[0]
	case "ios":
		return u.IOS[0]
	case "mac":
		return u.Mac[0]
	case "windows":
		return u.Windows[0]
	case "linux":
		return u.Linux[0]
	}
	return ""
}

type Agent struct {
	mux   sync.Mutex
	store map[string]UserAgent
}

func NewAgent() *Agent {
	a := Agent{
		mux:   sync.Mutex{},
		store: make(map[string]UserAgent),
	}
	return &a
}

func (a *Agent) Get(user, os string) string {
	data, ok := a.store[user]
	if !ok {
		return ""
	}
	return data.Get(os)
}
