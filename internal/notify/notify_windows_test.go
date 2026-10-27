package notify

import (
	"context"
	"errors"
	"syscall"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// The registration must hold the name and icon, and vanish at sign-out.
func TestRegisterIsVolatile(t *testing.T) {
	const id = "unsit.test"
	path := `Software\Classes\AppUserModelId\` + id
	t.Cleanup(func() { registry.DeleteKey(registry.CURRENT_USER, path) })

	register := func(icon string) registry.Key {
		t.Helper()
		if err := powershell(context.Background(), registerScript, "UNSIT_APP_ID="+id, "UNSIT_ICON="+icon); err != nil {
			t.Fatal(err)
		}
		k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.ALL_ACCESS)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { k.Close() })
		return k
	}

	// Quotes of any kind and brackets must pass through as they are.
	const icon = "C:\\it's \u2018here\u2019 [1]\\unsit.ico"
	k := register(icon)
	if name, _, err := k.GetStringValue("DisplayName"); err != nil || name != "unsit" {
		t.Errorf("DisplayName = %q, %v", name, err)
	}
	if got, _, err := k.GetStringValue("IconUri"); err != nil || got != icon {
		t.Errorf("IconUri = %q, %v; want %q", got, err, icon)
	}
	// Only a volatile key refuses a lasting subkey.
	const errorChildMustBeVolatile = syscall.Errno(1021)
	if _, _, err := registry.CreateKey(k, "lasting", registry.ALL_ACCESS); !errors.Is(err, errorChildMustBeVolatile) {
		t.Errorf("creating a lasting subkey: %v, want %v: the key is not volatile", err, errorChildMustBeVolatile)
	}

	k = register("")
	if _, _, err := k.GetStringValue("IconUri"); !errors.Is(err, registry.ErrNotExist) {
		t.Errorf("IconUri without an icon: %v, want it removed", err)
	}
}
