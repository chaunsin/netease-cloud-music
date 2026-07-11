// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"reflect"
	"testing"
)

func TestDefaultDomains(t *testing.T) {
	want := []string{
		"music.163.com",
		"music.126.net",
		"vod.126.net",
		"iplay.163.com",
		"look.163.com",
		"y.163.com",
		"163yun.com",
		"163jiasu.com",
		"netease.com",
		"acstatic-dun.126.net",
	}
	got := DefaultDomains()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultDomains() = %#v, want %#v", got, want)
	}

	got[0] = "changed.example"
	if DefaultDomains()[0] != want[0] {
		t.Fatal("DefaultDomains returned mutable package state")
	}
}

func TestHostMatcher(t *testing.T) {
	matcher, err := newHostMatcher(DefaultDomains())
	if err != nil {
		t.Fatalf("newHostMatcher() error = %v", err)
	}

	tests := []struct {
		host string
		want bool
	}{
		{host: "music.163.com", want: true},
		{host: "MUSIC.163.COM", want: true},
		{host: "music.163.com.", want: true},
		{host: "api.music.163.com", want: true},
		{host: "api.music.163.com.:443", want: true},
		{host: "look.163.com:80", want: true},
		{host: "cdn.netease.com", want: true},
		{host: "evilmusic.163.com", want: false},
		{host: "music.163.com.evil.example", want: false},
		{host: "reallynetease.com", want: false},
		{host: "netease.com.evil.example", want: false},
		{host: "", want: false},
		{host: ":443", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			if got := matcher.Match(tt.host); got != tt.want {
				t.Fatalf("Match(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

func TestNewHostMatcherRejectsEmptyDomains(t *testing.T) {
	for _, domains := range [][]string{nil, {}, {"music.163.com", " "}, {"."}} {
		if _, err := newHostMatcher(domains); err == nil {
			t.Fatalf("newHostMatcher(%q) unexpectedly succeeded", domains)
		}
	}
}

func TestNewHostMatcherNormalizesAndDeduplicatesDomains(t *testing.T) {
	matcher, err := newHostMatcher([]string{" Music.163.Com. ", "music.163.com", "look.163.com:443"})
	if err != nil {
		t.Fatalf("newHostMatcher() error = %v", err)
	}
	if want := []string{"music.163.com", "look.163.com"}; !reflect.DeepEqual(matcher.domains, want) {
		t.Fatalf("domains = %#v, want %#v", matcher.domains, want)
	}
}

func TestNilHostMatcherDoesNotMatch(t *testing.T) {
	var matcher *hostMatcher
	if matcher.Match("music.163.com") {
		t.Fatal("nil matcher unexpectedly matched host")
	}
}
