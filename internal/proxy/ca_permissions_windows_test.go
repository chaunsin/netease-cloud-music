// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

//go:build windows

package proxy

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowsCAPermissionsRestrictToCurrentUser(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proxy")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, "ca.key")
	if err := os.WriteFile(keyPath, []byte("private key"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := secureCAPrivateDir(dir); err != nil {
		t.Fatalf("secureCAPrivateDir() error = %v", err)
	}
	assertWindowsCurrentUserOnlyDACL(t, dir, windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE)

	if err := secureCAPrivateKey(keyPath); err != nil {
		t.Fatalf("secureCAPrivateKey() error = %v", err)
	}
	assertWindowsCurrentUserOnlyDACL(t, keyPath, windows.NO_INHERITANCE)
}

func assertWindowsCurrentUserOnlyDACL(t *testing.T, path string, inheritance uint32) {
	t.Helper()
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatalf("GetNamedSecurityInfo(%q): %v", path, err)
	}
	control, _, err := descriptor.Control()
	if err != nil {
		t.Fatalf("security descriptor control: %v", err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatalf("DACL for %q is not protected", path)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatalf("DACL for %q: %v", path, err)
	}
	if dacl.AceCount != 1 {
		t.Fatalf("DACL for %q has %d ACEs, want 1", path, dacl.AceCount)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatalf("GetAce(%q): %v", path, err)
	}
	if ace.Mask&windows.GENERIC_ALL == 0 {
		t.Fatalf("ACE for %q does not grant full control", path)
	}
	if uint32(ace.Header.AceFlags) != inheritance {
		t.Fatalf("ACE inheritance for %q = %#x, want %#x", path, ace.Header.AceFlags, inheritance)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatalf("GetTokenUser(): %v", err)
	}
	aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if !user.User.Sid.Equals(aceSID) {
		t.Fatalf("ACE for %q does not belong to the current user", path)
	}
}
