// MIT License
//
// Copyright (c) 2024 chaunsin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//

package api

import "sync"

type UserAgent struct {
	Android []string `json:"android" yaml:"android"`
	IOS     []string `json:"ios" yaml:"ios"`
	Mac     []string `json:"mac" yaml:"mac"`
	Windows []string `json:"windows" yaml:"windows"`
	Linux   []string `json:"linux" yaml:"linux"`
}

func (u *UserAgent) Get() string {
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

func (a *Agent) Get(user string) string {
	data, ok := a.store[user]
	if !ok {
		return ""
	}
	return data.Get()
}
