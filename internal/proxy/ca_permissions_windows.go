// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

//go:build windows

package proxy

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func secureCAPrivateKey(path string) error {
	return restrictCAPathToCurrentUser(path, windows.NO_INHERITANCE)
}

func secureCAPrivateDir(path string) error {
	return restrictCAPathToCurrentUser(path, windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE)
}

// restrictCAPathToCurrentUser replaces inherited permissions with a protected
// DACL granting full control only to the current process user. Windows file
// modes cannot express this policy, so every CA-key load reapplies it.
func restrictCAPathToCurrentUser(path string, inheritance uint32) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("get current Windows user SID: %w", err)
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       inheritance,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
		},
	}}, nil)
	if err != nil {
		return fmt.Errorf("build private CA ACL: %w", err)
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		acl,
		nil,
	); err != nil {
		return fmt.Errorf("set private CA ACL: %w", err)
	}
	return nil
}
